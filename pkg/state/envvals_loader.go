package state

import (
	"fmt"
	"path/filepath"
	"strings"

	"dario.cat/mergo"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/hcllang"
	"github.com/helmfile/helmfile/pkg/maputil"
	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/tmpl"
	"github.com/helmfile/helmfile/pkg/yaml"
)

type EnvironmentValuesLoader struct {
	storage *Storage

	fs *filesystem.FileSystem

	logger *zap.SugaredLogger

	remote *remote.Remote
}

func NewEnvironmentValuesLoader(storage *Storage, fs *filesystem.FileSystem, logger *zap.SugaredLogger, remote *remote.Remote) *EnvironmentValuesLoader {
	return &EnvironmentValuesLoader{
		storage: storage,
		fs:      fs,
		logger:  logger,
		remote:  remote,
	}
}

func (ld *EnvironmentValuesLoader) LoadEnvironmentValues(missingFileHandler *string, valuesEntries []any, ctxEnv *environment.Environment, envName string) (map[string]any, error) {
	var (
		result    = map[string]any{}
		hclLoader = hcllang.NewHCLLoader(ld.fs, ld.logger)
		err       error
	)

	for _, entry := range valuesEntries {
		maps := []any{}

		switch strOrMap := entry.(type) {
		case string:
			files, skipped, err := ld.storage.resolveFile(missingFileHandler, "environment values", entry.(string))
			if err != nil {
				return nil, err
			}
			if skipped {
				continue
			}
			for _, f := range files {
				var env environment.Environment
				if ctxEnv == nil {
					env = *environment.New(envName)
				} else {
					env = *ctxEnv
				}
				if strings.HasSuffix(f, ".hcl") {
					hclLoader.AddFile(f)
				} else {
					tmplData := NewEnvironmentTemplateData(env, "", env.Values)
					r := tmpl.NewFileRenderer(ld.fs, filepath.Dir(f), tmplData)
					bytes, err := r.RenderToBytes(f)
					if err != nil {
						return nil, fmt.Errorf("failed to load environment values file \"%s\": %v", f, err)
					}
					m := map[string]any{}
					if err := yaml.Unmarshal(bytes, &m); err != nil {
						return nil, fmt.Errorf("failed to load environment values file \"%s\": %v\n\nOffending YAML:\n%s", f, err, bytes)
					}
					maps = append(maps, m)
					ld.logger.Debugf("envvals_loader: loaded %s:%v", strOrMap, m)
				}
			}
		case map[any]any, map[string]any:
			// Render template expressions in inline map values
			// This allows users to use template functions like {{ env "VAR" }} or {{ readFile "file" }}
			// in inline values passed to sub-helmfiles
			var env environment.Environment
			if ctxEnv == nil {
				env = *environment.New(envName)
			} else {
				env = *ctxEnv
			}
			tmplData := NewEnvironmentTemplateData(env, "", env.Values)
			r := tmpl.NewTextRenderer(ld.fs, ld.storage.basePath, tmplData)

			// Change working directory to basePath for template rendering
			// This is needed because some template functions like fetchSecretValue
			// resolve relative paths based on the current working directory
			renderedMap, err := ld.renderInBasePath(strOrMap, r)
			if err != nil {
				return nil, fmt.Errorf("failed to render inline values: %v", err)
			}
			maps = append(maps, renderedMap)
		default:
			return nil, fmt.Errorf("unexpected type of value: value=%v, type=%T", strOrMap, strOrMap)
		}

		result, err = mapMerge(result, maps)
		if err != nil {
			return nil, err
		}
	}
	maps := []any{}
	if hclLoader.Length() > 0 {
		m, err := hclLoader.HCLRender()
		if err != nil {
			return nil, err
		}
		maps = append(maps, m)
	}
	result, err = mapMerge(result, maps)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// renderInBasePath temporarily changes the working directory to basePath before rendering
// template expressions. This is needed because some template functions like fetchSecretValue
// resolve relative paths based on the current working directory.
func (ld *EnvironmentValuesLoader) renderInBasePath(value any, r tmpl.TextRenderer) (any, error) {
	// If filesystem doesn't support Getwd/Chdir (e.g., in tests), just render without changing directory
	if ld.fs.Getwd == nil || ld.fs.Chdir == nil {
		return ld.renderTemplateExpressions(value, r)
	}

	// Save current working directory using the filesystem abstraction
	cwd, err := ld.fs.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %v", err)
	}

	// Change to basePath for rendering using the filesystem abstraction
	if err := ld.fs.Chdir(ld.storage.basePath); err != nil {
		return nil, fmt.Errorf("failed to change to base directory %q: %v", ld.storage.basePath, err)
	}

	// Ensure we change back to the original directory
	defer func() {
		if err := ld.fs.Chdir(cwd); err != nil {
			ld.logger.Warnf("failed to change back to original directory %q: %v", cwd, err)
		}
	}()

	return ld.renderTemplateExpressions(value, r)
}

// renderTemplateExpressions recursively renders template expressions in map values.
// This allows users to use template functions like {{ env "VAR" }}, {{ readFile "file" }},
// {{ fetchSecretValue "ref+..." }}, etc. in inline values passed to sub-helmfiles.
func (ld *EnvironmentValuesLoader) renderTemplateExpressions(value any, r tmpl.TextRenderer) (any, error) {
	switch v := value.(type) {
	case string:
		// Check if the string contains template expressions (has {{ and }})
		if strings.Contains(v, "{{") && strings.Contains(v, "}}") {
			rendered, err := r.RenderTemplateText(v)
			if err != nil {
				return nil, fmt.Errorf("failed to render template expression %q: %v", v, err)
			}
			return rendered, nil
		}
		return v, nil
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			renderedVal, err := ld.renderTemplateExpressions(val, r)
			if err != nil {
				return nil, err
			}
			result[key] = renderedVal
		}
		return result, nil
	case map[any]any:
		result := make(map[any]any, len(v))
		for key, val := range v {
			renderedVal, err := ld.renderTemplateExpressions(val, r)
			if err != nil {
				return nil, err
			}
			result[key] = renderedVal
		}
		return result, nil
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			renderedItem, err := ld.renderTemplateExpressions(item, r)
			if err != nil {
				return nil, err
			}
			result[i] = renderedItem
		}
		return result, nil
	default:
		// For other types (int, bool, etc.), return as-is
		return v, nil
	}
}

func mapMerge(dest map[string]any, maps []any) (map[string]any, error) {
	for _, m := range maps {
		// All the nested map key should be string. Otherwise we get strange errors due to that
		// mergo or reflect is unable to merge map[any]any with map[string]any or vice versa.
		// See https://github.com/roboll/helmfile/issues/677
		vals, err := maputil.CastKeysToStrings(m)
		if err != nil {
			return nil, err
		}
		if err := mergo.Merge(&dest, &vals, mergo.WithOverride); err != nil {
			return nil, fmt.Errorf("failed to merge %v: %v", m, err)
		}
	}
	return dest, nil
}
