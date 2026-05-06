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

func (ld *EnvironmentValuesLoader) LoadEnvironmentValues(missingFileHandler *string, valuesEntries []any, ctxEnv *environment.Environment, envName string, mergeStrategy string) (map[string]any, error) {
	switch mergeStrategy {
	case "", MergeStrategyOverride, MergeStrategyFallback:
	default:
		return nil, fmt.Errorf("environment %q: invalid mergeStrategy %q (must be %q or %q)",
			envName, mergeStrategy, MergeStrategyOverride, MergeStrategyFallback)
	}
	var (
		result    = map[string]any{}
		hclLoader = hcllang.NewHCLLoader(ld.fs, ld.logger)
		err       error
	)

	for _, entry := range valuesEntries {
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
					continue
				}
				// Use merged values (Defaults + Values + CLIOverrides) for template rendering
				// so that CLI values are accessible via .Values in environment value files.
				mergedVals, err := env.GetMergedValues()
				if err != nil {
					return nil, fmt.Errorf("failed to get merged values for environment file \"%s\": %v", f, err)
				}
				// Under fallback strategy, also expose values accumulated from earlier files
				// in this same `values:` list, including earlier files in this same glob
				// expansion, so a later .gotmpl can reference them via .Values (e.g.
				// `{{ .Values.cluster.domain }}`). Env CLI overrides and values still win,
				// layered on top with WithOverride.
				if mergeStrategy == MergeStrategyFallback && len(result) > 0 {
					enriched := map[string]any{}
					if err := mergo.Merge(&enriched, result); err != nil {
						return nil, fmt.Errorf("failed to build template context for \"%s\": %v", f, err)
					}
					if err := mergo.Merge(&enriched, mergedVals, mergo.WithOverride); err != nil {
						return nil, fmt.Errorf("failed to build template context for \"%s\": %v", f, err)
					}
					mergedVals = enriched
				}
				tmplData := NewEnvironmentTemplateData(env, "", mergedVals)
				r := tmpl.NewFileRenderer(ld.fs, filepath.Dir(f), tmplData)
				bytes, err := r.RenderToBytes(f)
				if err != nil {
					return nil, fmt.Errorf("failed to load environment values file \"%s\": %v", f, err)
				}
				m := map[string]any{}
				if err := yaml.Unmarshal(bytes, &m); err != nil {
					return nil, fmt.Errorf("failed to load environment values file \"%s\": %v\n\nOffending YAML:\n%s", f, err, bytes)
				}
				ld.logger.Debugf("envvals_loader: loaded %s:%v", strOrMap, m)
				// Merge each file into result immediately so subsequent files in the same
				// entry's expansion (e.g. a glob) can see prior files' values via .Values
				// when rendered as templates.
				result, err = mapMerge(result, []any{m}, mergeStrategy)
				if err != nil {
					return nil, err
				}
			}
		case map[any]any, map[string]any:
			result, err = mapMerge(result, []any{strOrMap}, mergeStrategy)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unexpected type of value: value=%v, type=%T", strOrMap, strOrMap)
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
	result, err = mapMerge(result, maps, mergeStrategy)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func mapMerge(dest map[string]any, maps []any, mergeStrategy string) (map[string]any, error) {
	for _, m := range maps {
		// All the nested map key should be string. Otherwise we get strange errors due to that
		// mergo or reflect is unable to merge map[any]any with map[string]any or vice versa.
		// See https://github.com/roboll/helmfile/issues/677
		vals, err := maputil.CastKeysToStrings(m)
		if err != nil {
			return nil, err
		}
		if mergeStrategy == MergeStrategyFallback {
			// First-file-wins: the new file is the base and the
			// accumulator overlays it, so keys already accumulated keep
			// their value while keys only present in the new file fill
			// in. MergeMaps is used instead of mergo because mergo's
			// isEmptyValue rule would silently let a later fallback's
			// `enabled: true` clobber an explicit `enabled: false`.
			dest = maputil.MergeMaps(vals, dest)
			continue
		}
		if err := mergo.Merge(&dest, &vals, mergo.WithOverride); err != nil {
			return nil, fmt.Errorf("failed to merge %v: %v", m, err)
		}
	}
	return dest, nil
}
