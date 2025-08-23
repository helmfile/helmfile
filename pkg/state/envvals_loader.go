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
			maps = append(maps, strOrMap)
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
