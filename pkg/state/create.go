package state

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"dario.cat/mergo"
	"github.com/helmfile/vals"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/maputil"
	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/yaml"
)

const (
	DefaultHelmBinary       = "helm"
	DefaultKustomizeBinary  = "kustomize"
	DefaultHCLFileExtension = ".hcl"
)

var ValidUpdateStrategyValues = []string{UpdateStrategyReinstallIfForbidden}

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

type InvalidUpdateStrategyError struct {
	UpdateStrategy string
}

func (e *InvalidUpdateStrategyError) Error() string {
	return fmt.Sprintf("updateStrategy %q is invalid, valid values are: %s or not set", e.UpdateStrategy, strings.Join(ValidUpdateStrategyValues, ", "))
}

type StateCreator struct {
	logger *zap.SugaredLogger

	fs *filesystem.FileSystem

	valsRuntime vals.Evaluator

	Strict bool

	LoadFile func(inheritedEnv, overrodeEnv *environment.Environment, baseDir, file string, evaluateBases bool) (*HelmState, error)

	getHelm func(*HelmState) (helmexec.Interface, error)

	overrideHelmBinary string

	overrideKustomizeBinary string

	enableLiveOutput bool

	remote *remote.Remote

	lockFile string
}

func NewCreator(logger *zap.SugaredLogger, fs *filesystem.FileSystem, valsRuntime vals.Evaluator, getHelm func(*HelmState) (helmexec.Interface, error), overrideHelmBinary string, overrideKustomizeBinary string, remote *remote.Remote, enableLiveOutput bool, lockFile string) *StateCreator {
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
			if filepath.Ext(file) != ".gotmpl" {
				return nil, &StateLoadError{fmt.Sprintf("failed to read %s: reading document at index %d. Started seeing this since Helmfile v1? Add the .gotmpl file extension", file, i), err}
			}
			return nil, &StateLoadError{fmt.Sprintf("failed to read %s: reading document at index %d", file, i), err}
		}

		if err := mergo.Merge(&state, &intermediate, mergo.WithAppendSlice, mergo.WithOverride); err != nil {
			return nil, &StateLoadError{fmt.Sprintf("failed to read %s: merging document at index %d", file, i), err}
		}
	}

	state.logger = c.logger
	state.valsRuntime = c.valsRuntime

	return &state, nil
}

// applyDefaultsAndOverrides applies default binary paths and command-line overrides
func (c *StateCreator) applyDefaultsAndOverrides(state *HelmState) {
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
}

// LoadEnvValues loads environment values files relative to the `baseDir`
func (c *StateCreator) LoadEnvValues(target *HelmState, env string, failOnMissingEnv bool, ctxEnv, overrode *environment.Environment) (*HelmState, error) {
	state := *target

	e, err := c.loadEnvValues(&state, env, failOnMissingEnv, ctxEnv, overrode)
	if err != nil {
		return nil, &StateLoadError{fmt.Sprintf("failed to read %s", state.FilePath), err}
	}

	newDefaults, err := state.loadValuesEntries(nil, state.DefaultValues, c.remote, ctxEnv, env)
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

		// Apply default binaries and command-line overrides only for the main helmfile
		// after loading and merging all bases. This ensures that values from bases are
		// properly respected and that later bases/documents can override earlier ones.
		c.applyDefaultsAndOverrides(state)
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

// mergeEnvironments deeply merges environment specifications from src into dst.
// Unlike mergo.WithOverride which replaces entire EnvironmentSpec values, this function
// properly merges the Values slices from both environments.
func mergeEnvironments(dst, src map[string]EnvironmentSpec) {
	// If dst is nil, there's nothing to merge into
	if dst == nil {
		return
	}

	for envName, srcEnv := range src {
		if dstEnv, exists := dst[envName]; exists {
			// Environment exists in both - merge the Values slices
			mergedValues := append([]any{}, dstEnv.Values...)
			mergedValues = append(mergedValues, srcEnv.Values...)

			// Merge Secrets slices
			mergedSecrets := append([]string{}, dstEnv.Secrets...)
			mergedSecrets = append(mergedSecrets, srcEnv.Secrets...)

			// Create merged environment
			merged := EnvironmentSpec{
				Values:  mergedValues,
				Secrets: mergedSecrets,
			}

			// Override KubeContext if src has it
			if srcEnv.KubeContext != "" {
				merged.KubeContext = srcEnv.KubeContext
			} else {
				merged.KubeContext = dstEnv.KubeContext
			}

			// Override MissingFileHandler if src has it
			if srcEnv.MissingFileHandler != nil {
				merged.MissingFileHandler = srcEnv.MissingFileHandler
			} else {
				merged.MissingFileHandler = dstEnv.MissingFileHandler
			}

			// Override MissingFileHandlerConfig if src has it
			if srcEnv.MissingFileHandlerConfig != nil {
				merged.MissingFileHandlerConfig = srcEnv.MissingFileHandlerConfig
			} else {
				merged.MissingFileHandlerConfig = dstEnv.MissingFileHandlerConfig
			}

			dst[envName] = merged
		} else {
			// Environment only exists in src - just copy it
			dst[envName] = srcEnv
		}
	}
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
		// Initialize Environments map if nil to avoid panic in mergeEnvironments
		if layers[0].Environments == nil {
			layers[0].Environments = make(map[string]EnvironmentSpec)
		}

		// Manually merge environments to ensure deep merging of environment values
		mergeEnvironments(layers[0].Environments, layers[i].Environments)

		// Clear the Environments from the source before mergo to avoid override
		tmpEnvs := layers[i].Environments
		layers[i].Environments = nil

		// Now merge the rest of the fields
		if err := mergo.Merge(layers[0], layers[i], mergo.WithAppendSlice, mergo.WithOverride); err != nil {
			return nil, err
		}

		// Restore the Environments back to the source layer (in case it's used later)
		layers[i].Environments = tmpEnvs
	}

	return layers[0], nil
}

// getEnvMissingFileHandlerConfig returns the first non-nil MissingFileHandlerConfig from the environment spec, state, or default.
func (st *HelmState) getEnvMissingFileHandlerConfig(es EnvironmentSpec) *MissingFileHandlerConfig {
	switch {
	case es.MissingFileHandlerConfig != nil:
		return es.MissingFileHandlerConfig
	case st.MissingFileHandlerConfig != nil:
		return st.MissingFileHandlerConfig
	default:
		return nil
	}
}

// getEnvMissingFileHandler returns the first non-nil MissingFileHandler from the environment spec, state, or default.
func (st *HelmState) getEnvMissingFileHandler(es EnvironmentSpec) *string {
	defaultMissingFileHandler := "Error"
	switch {
	case es.MissingFileHandler != nil:
		return es.MissingFileHandler
	case st.MissingFileHandler != nil:
		return st.MissingFileHandler
	default:
		return &defaultMissingFileHandler
	}
}

// nolint: unparam
func (c *StateCreator) loadEnvValues(st *HelmState, name string, failOnMissingEnv bool, ctxEnv, overrode *environment.Environment) (*environment.Environment, error) {
	secretVals := map[string]any{}
	valuesVals := map[string]any{}
	envSpec, ok := st.Environments[name]
	decryptedFiles := []string{}
	if ok {
		var err error
		// To keep supporting the secrets entries having precedence over the values
		// This require to be done in 2 steps for HCL encrypted file support :
		// 1. Get the Secrets
		// 2. Merge the secrets with the envValues after
		// Also makes the fail +- faster as it's trying to decrypt before loading values
		var envSecretFiles []string
		if len(envSpec.Secrets) > 0 {
			for _, urlOrPath := range envSpec.Secrets {
				resolved, skipped, err := st.storage().resolveFile(st.getEnvMissingFileHandler(envSpec), "environment values", urlOrPath, st.getEnvMissingFileHandlerConfig(envSpec).resolveFileOptions()...)
				if err != nil {
					return nil, err
				}
				if skipped {
					continue
				}
				envSecretFiles = append(envSecretFiles, resolved...)
			}
			keepSecretFilesExtensions := []string{DefaultHCLFileExtension}
			decryptedFiles, err = c.scatterGatherEnvSecretFiles(st, envSecretFiles, secretVals, keepSecretFilesExtensions)
			if err != nil {
				return nil, err
			}

			defer func() {
				for _, file := range decryptedFiles {
					if err := c.fs.DeleteFile(file); err != nil {
						c.logger.Warnf("failed removing decrypted file %s: %w", file, err)
					}
				}
			}()
		}
		var valuesFiles []any
		for _, f := range decryptedFiles {
			valuesFiles = append(valuesFiles, f)
		}
		envValuesEntries := append(valuesFiles, envSpec.Values...)
		loadValuesEntriesEnv, err := ctxEnv.Merge(overrode)
		if err != nil {
			return nil, err
		}
		valuesVals, err = st.loadValuesEntries(envSpec.MissingFileHandler, envValuesEntries, c.remote, loadValuesEntriesEnv, name)
		if err != nil {
			return nil, err
		}

		if err = mergo.Merge(&valuesVals, &secretVals, mergo.WithOverride); err != nil {
			return nil, err
		}
	} else if ctxEnv == nil && overrode == nil && name != DefaultEnv && failOnMissingEnv {
		return nil, &UndefinedEnvError{Env: name}
	}

	newEnv := &environment.Environment{Name: name, Values: valuesVals, KubeContext: envSpec.KubeContext}

	if ctxEnv != nil {
		intCtxEnv := *ctxEnv

		if err := mergo.Merge(&intCtxEnv, newEnv, mergo.WithOverride); err != nil {
			return nil, fmt.Errorf("error while merging environment values for \"%s\": %v", name, err)
		}

		newEnv = &intCtxEnv
	}

	if overrode != nil {
		intOverrodeEnv := *newEnv

		// Use MergeMaps instead of mergo.Merge to properly handle array merging element-by-element
		// This fixes issue #2281 where arrays were being replaced entirely instead of merged
		intOverrodeEnv.Values = maputil.MergeMaps(intOverrodeEnv.Values, overrode.Values)

		newEnv = &intOverrodeEnv
	}

	return newEnv, nil
}

// For all keepFileExtensions, the decrypted files will be retained
// with the specified extensions
// They will not be parsed nor added to the envVals.
// Only their decrypted filePath will be returned
// Up to the caller to remove them
func (c *StateCreator) scatterGatherEnvSecretFiles(st *HelmState, envSecretFiles []string, envVals map[string]any, keepFileExtensions []string) ([]string, error) {
	var errs []error
	var decryptedFilesKeeper []string
	helm, err := c.getHelm(st)
	if err != nil {
		return nil, err
	}
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
				for _, ext := range keepFileExtensions {
					if strings.HasSuffix(secret.path, ext) {
						decryptedFilesKeeper = append(decryptedFilesKeeper, decFile)
					}
				}
				// nolint: staticcheck
				defer func() {
					if !slices.Contains(decryptedFilesKeeper, decFile) {
						if err := c.fs.DeleteFile(decFile); err != nil {
							c.logger.Warnf("removing decrypted file %s: %w", decFile, err)
						}
					}
				}()
				var vals map[string]any
				if !slices.Contains(decryptedFilesKeeper, decFile) {
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
					vals, err = maputil.CastKeysToStrings(m)
					if err != nil {
						results <- secretResult{secret.id, nil, fmt.Errorf("failed to load environment secrets file \"%s\": %v", secret.path, err), secret.path}
						continue
					}
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
		return decryptedFilesKeeper, fmt.Errorf("failed loading environment secrets with %d errors", len(errs))
	}
	return decryptedFilesKeeper, nil
}

func (st *HelmState) loadValuesEntries(missingFileHandler *string, entries []any, remote *remote.Remote, ctxEnv *environment.Environment, envName string) (map[string]any, error) {
	var envVals map[string]any

	valuesEntries := append([]any{}, entries...)
	ld := NewEnvironmentValuesLoader(st.storage(), st.fs, st.logger, remote)
	var err error
	envVals, err = ld.LoadEnvironmentValues(missingFileHandler, valuesEntries, ctxEnv, envName)
	if err != nil {
		return nil, err
	}

	return envVals, nil
}
