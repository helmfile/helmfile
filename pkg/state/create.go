package state

import (
	"errors"
	"fmt"
	"io"

	"github.com/helmfile/vals"
	"github.com/imdario/mergo"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/maputil"
	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/yaml"
)

const (
	DefaultHelmBinary      = "helm"
	DefaultKustomizeBinary = "kustomize"
)

type StateLoadError struct {
	Msg   string
	Cause error
}

func (e *StateLoadError) Error() string {
	return fmt.Sprintf("%s: %v", e.Msg, e.Cause)
}

type UndefinedEnvError struct {
	Env string
}

func (e *UndefinedEnvError) Error() string {
	return fmt.Sprintf("environment \"%s\" is not defined", e.Env)
}

type StateCreator struct {
	logger *zap.SugaredLogger

	fs *filesystem.FileSystem

	valsRuntime vals.Evaluator

	Strict bool

	LoadFile func(inheritedEnv, overrodeEnv *environment.Environment, baseDir, file string, evaluateBases bool) (*HelmState, error)

	getHelm func(*HelmState) helmexec.Interface

	overrideHelmBinary string

	overrideKustomizeBinary string

	enableLiveOutput bool

	remote *remote.Remote

	lockFile string
}

func NewCreator(logger *zap.SugaredLogger, fs *filesystem.FileSystem, valsRuntime vals.Evaluator, getHelm func(*HelmState) helmexec.Interface, overrideHelmBinary string, overrideKustomizeBinary string, remote *remote.Remote, enableLiveOutput bool, lockFile string) *StateCreator {
	return &StateCreator{
		logger: logger,

		Strict:      true,
		fs:          fs,
		valsRuntime: valsRuntime,
		getHelm:     getHelm,

		overrideHelmBinary:      overrideHelmBinary,
		overrideKustomizeBinary: overrideKustomizeBinary,
		enableLiveOutput:        enableLiveOutput,

		remote: remote,

		lockFile: lockFile,
	}
}

// Parse parses YAML into HelmState
func (c *StateCreator) Parse(content []byte, baseDir, file string) (*HelmState, error) {
	var state HelmState

	state.fs = c.fs
	state.FilePath = file
	state.basePath = baseDir

	state.LockFile = c.lockFile

	decode := yaml.NewDecoder(content, c.Strict)

	i := 0
	for {
		i++

		var intermediate HelmState

		intermediate.FilePath = file

		err := decode(&intermediate)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, &StateLoadError{fmt.Sprintf("failed to read %s: reading document at index %d", file, i), err}
		}

		if err := mergo.Merge(&state, &intermediate, mergo.WithAppendSlice); err != nil {
			return nil, &StateLoadError{fmt.Sprintf("failed to read %s: merging document at index %d", file, i), err}
		}
	}

	// TODO: Remove this function once Helmfile v0.x
	if len(state.DeprecatedReleases) > 0 {
		if len(state.Releases) > 0 {
			return nil, fmt.Errorf("failed to parse %s: you can't specify both `charts` and `releases` sections", file)
		}
		state.Releases = state.DeprecatedReleases
		state.DeprecatedReleases = []ReleaseSpec{}
	}

	// TODO: Remove this function once Helmfile v0.x
	if state.DeprecatedContext != "" && state.HelmDefaults.KubeContext == "" {
		state.HelmDefaults.KubeContext = state.DeprecatedContext
	}

	if c.overrideHelmBinary != "" && c.overrideHelmBinary != DefaultHelmBinary {
		state.DefaultHelmBinary = c.overrideHelmBinary
	} else if state.DefaultHelmBinary == "" {
		// Let `helmfile --helm-binary ""` not break this helmfile run
		state.DefaultHelmBinary = DefaultHelmBinary
	}

	if c.overrideKustomizeBinary != "" && c.overrideKustomizeBinary != DefaultKustomizeBinary {
		state.DefaultKustomizeBinary = c.overrideKustomizeBinary
	} else if state.DefaultKustomizeBinary == "" {
		// Let `helmfile --kustomize-binary ""` not break this helmfile run
		state.DefaultKustomizeBinary = DefaultKustomizeBinary
	}

	state.logger = c.logger
	state.valsRuntime = c.valsRuntime

	return &state, nil
}

// LoadEnvValues loads environment values files relative to the `baseDir`
func (c *StateCreator) LoadEnvValues(target *HelmState, env string, failOnMissingEnv bool, ctxEnv, overrode *environment.Environment) (*HelmState, error) {
	state := *target

	e, err := c.loadEnvValues(&state, env, failOnMissingEnv, ctxEnv, overrode)
	if err != nil {
		return nil, &StateLoadError{fmt.Sprintf("failed to read %s", state.FilePath), err}
	}

	newDefaults, err := state.loadValuesEntries(nil, state.DefaultValues, nil, c.remote, ctxEnv, env)
	if err != nil {
		return nil, err
	}

	if err := mergo.Merge(&e.Defaults, newDefaults, mergo.WithOverride); err != nil {
		return nil, err
	}

	state.Env = *e

	return &state, nil
}

// Parses YAML into HelmState, while loading environment values files relative to the `baseDir`
// evaluateBases=true means that this is NOT a base helmfile
func (c *StateCreator) ParseAndLoad(content []byte, baseDir, file string, envName string, failOnMissingEnv, evaluateBases bool, envValues, overrode *environment.Environment) (*HelmState, error) {
	state, err := c.Parse(content, baseDir, file)
	if err != nil {
		return nil, err
	}

	if !evaluateBases {
		if len(state.Bases) > 0 {
			return nil, errors.New("nested `base` helmfile is unsupported. please submit a feature request if you need this!")
		}
	} else {
		state, err = c.loadBases(envValues, overrode, state, baseDir)
		if err != nil {
			return nil, err
		}
	}

	state, err = c.LoadEnvValues(state, envName, failOnMissingEnv, envValues, overrode)
	if err != nil {
		return nil, err
	}

	state.FilePath = file

	vals, err := state.Env.GetMergedValues()
	if err != nil {
		return nil, fmt.Errorf("rendering values: %w", err)
	}
	state.RenderedValues = vals

	return state, nil
}

func (c *StateCreator) loadBases(envValues, overrodeEnv *environment.Environment, st *HelmState, baseDir string) (*HelmState, error) {
	var newOverrodeEnv *environment.Environment
	if overrodeEnv != nil {
		overrodeEnvCopier := overrodeEnv.DeepCopy()
		newOverrodeEnv = &overrodeEnvCopier
	}
	layers := []*HelmState{}
	for _, b := range st.Bases {
		base, err := c.LoadFile(envValues, newOverrodeEnv, baseDir, b, false)
		if err != nil {
			return nil, err
		}
		layers = append(layers, base)
	}
	layers = append(layers, st)

	for i := 1; i < len(layers); i++ {
		if err := mergo.Merge(layers[0], layers[i], mergo.WithAppendSlice); err != nil {
			return nil, err
		}
	}

	return layers[0], nil
}

// nolint: unparam
func (c *StateCreator) loadEnvValues(st *HelmState, name string, failOnMissingEnv bool, ctxEnv, overrode *environment.Environment) (*environment.Environment, error) {
	envVals := map[string]any{}
	envSpec, ok := st.Environments[name]
	if ok {
		var err error
		envVals, err = st.loadValuesEntries(envSpec.MissingFileHandler, envSpec.Values, envSpec.LayeredValues, c.remote, ctxEnv, name)
		if err != nil {
			return nil, err
		}

		if len(envSpec.Secrets) > 0 {
			var envSecretFiles []string
			for _, urlOrPath := range envSpec.Secrets {
				resolved, skipped, err := st.storage().resolveFile(envSpec.MissingFileHandler, "environment values", urlOrPath, envSpec.MissingFileHandlerConfig.resolveFileOptions()...)
				if err != nil {
					return nil, err
				}
				if skipped {
					continue
				}

				envSecretFiles = append(envSecretFiles, resolved...)
			}
			if err = c.scatterGatherEnvSecretFiles(st, envSecretFiles, envVals); err != nil {
				return nil, err
			}
		}
	} else if ctxEnv == nil && overrode == nil && name != DefaultEnv && failOnMissingEnv {
		return nil, &UndefinedEnvError{Env: name}
	}

	newEnv := &environment.Environment{Name: name, Values: envVals, KubeContext: envSpec.KubeContext}

	if ctxEnv != nil {
		intCtxEnv := *ctxEnv

		if err := mergo.Merge(&intCtxEnv, newEnv, mergo.WithOverride); err != nil {
			return nil, fmt.Errorf("error while merging environment values for \"%s\": %v", name, err)
		}

		newEnv = &intCtxEnv
	}

	if overrode != nil {
		intOverrodeEnv := *newEnv

		if err := mergo.Merge(&intOverrodeEnv, overrode, mergo.WithOverride); err != nil {
			return nil, fmt.Errorf("error while merging environment overrode values for \"%s\": %v", name, err)
		}

		newEnv = &intOverrodeEnv
	}

	return newEnv, nil
}

func (c *StateCreator) scatterGatherEnvSecretFiles(st *HelmState, envSecretFiles []string, envVals map[string]any) error {
	var errs []error

	helm := c.getHelm(st)
	inputs := envSecretFiles
	inputsSize := len(inputs)

	type secretResult struct {
		id     int
		result map[string]any
		err    error
		path   string
	}

	type secretInput struct {
		id   int
		path string
	}

	secrets := make(chan secretInput, inputsSize)
	results := make(chan secretResult, inputsSize)

	st.scatterGather(0, inputsSize,
		func() {
			for i, secretFile := range envSecretFiles {
				secrets <- secretInput{i, secretFile}
			}
			close(secrets)
		},
		func(id int) {
			for secret := range secrets {
				release := &ReleaseSpec{}
				decFile, err := helm.DecryptSecret(st.createHelmContext(release, 0), secret.path)
				if err != nil {
					results <- secretResult{secret.id, nil, err, secret.path}
					continue
				}

				// nolint: staticcheck
				defer func() {
					if err := c.fs.DeleteFile(decFile); err != nil {
						c.logger.Warnf("removing decrypted file %s: %w", decFile, err)
					}
				}()

				bytes, err := c.fs.ReadFile(decFile)
				if err != nil {
					results <- secretResult{secret.id, nil, fmt.Errorf("failed to load environment secrets file \"%s\": %v", secret.path, err), secret.path}
					continue
				}
				m := map[string]any{}
				if err := yaml.Unmarshal(bytes, &m); err != nil {
					results <- secretResult{secret.id, nil, fmt.Errorf("failed to load environment secrets file \"%s\": %v", secret.path, err), secret.path}
					continue
				}
				// All the nested map key should be string. Otherwise we get strange errors due to that
				// mergo or reflect is unable to merge map[any]any with map[string]any or vice versa.
				// See https://github.com/roboll/helmfile/issues/677
				vals, err := maputil.CastKeysToStrings(m)
				if err != nil {
					results <- secretResult{secret.id, nil, fmt.Errorf("failed to load environment secrets file \"%s\": %v", secret.path, err), secret.path}
					continue
				}
				results <- secretResult{secret.id, vals, nil, secret.path}
			}
		},
		func() {
			sortedSecrets := make([]secretResult, inputsSize)

			for i := 0; i < inputsSize; i++ {
				result := <-results
				sortedSecrets[result.id] = result
			}
			close(results)

			for _, result := range sortedSecrets {
				if result.err != nil {
					errs = append(errs, result.err)
				} else {
					if err := mergo.Merge(&envVals, &result.result, mergo.WithOverride); err != nil {
						errs = append(errs, fmt.Errorf("failed to load environment secrets file \"%s\": %v", result.path, err))
					}
				}
			}
		},
	)

	if len(errs) > 0 {
		for _, err := range errs {
			st.logger.Error(err)
		}
		return fmt.Errorf("failed loading environment secrets with %d errors", len(errs))
	}
	return nil
}

func (st *HelmState) loadValuesEntries(missingFileHandler *string, entries []any, layeredEntries []any, remote *remote.Remote, ctxEnv *environment.Environment, envName string) (map[string]any, error) {
	envVals := make(map[string]any, len(entries)+len(layeredEntries))

	ld := NewEnvironmentValuesLoader(st.storage(), st.fs, st.logger, remote)

	if entries != nil {
		valuesEntries := append([]any{}, entries...)
		if err := ld.LoadEnvironmentValues(missingFileHandler, valuesEntries, ctxEnv, envName, envVals, false); err != nil {
			return nil, err
		}
	}

	if layeredEntries != nil {
		layeredValuesEntries := append([]any{}, layeredEntries...)
		if err := ld.LoadEnvironmentValues(missingFileHandler, layeredValuesEntries, ctxEnv, envName, envVals, true); err != nil {
			return nil, err
		}
	}

	return envVals, nil
}
