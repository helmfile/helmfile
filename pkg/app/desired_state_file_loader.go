package app

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/helmfile/vals"
	"github.com/imdario/mergo"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/runtime"
	"github.com/helmfile/helmfile/pkg/state"
)

const (
	DefaultHelmBinary = state.DefaultHelmBinary
)

type desiredStateLoader struct {
	overrideKubeContext string
	overrideHelmBinary  string
	enableLiveOutput    bool

	env       string
	namespace string
	chart     string
	fs        *filesystem.FileSystem

	getHelm func(*state.HelmState) helmexec.Interface

	remote      *remote.Remote
	logger      *zap.SugaredLogger
	valsRuntime vals.Evaluator

	lockFilePath string
}

func (ld *desiredStateLoader) Load(f string, opts LoadOpts) (*state.HelmState, error) {
	var overrodeEnv *environment.Environment

	args := opts.Environment.OverrideValues

	if len(args) > 0 {
		if opts.CalleePath == "" {
			return nil, fmt.Errorf("bug: opts.CalleePath was nil: f=%s, opts=%v", f, opts)
		}
		storage := state.NewStorage(opts.CalleePath, ld.logger, ld.fs)
		envld := state.NewEnvironmentValuesLoader(storage, ld.fs, ld.logger, ld.remote)
		handler := state.MissingFileHandlerError
		vals, err := envld.LoadEnvironmentValues(&handler, args, environment.New(ld.env), ld.env)
		if err != nil {
			return nil, err
		}

		overrodeEnv = &environment.Environment{
			Name:   ld.env,
			Values: vals,
		}
	}

	st, err := ld.loadFileWithOverrides(nil, overrodeEnv, filepath.Dir(f), filepath.Base(f), true)
	if err != nil {
		return nil, err
	}

	if opts.Reverse {
		st.Reverse()
	}

	if ld.overrideKubeContext != "" {
		if st.OverrideKubeContext != "" {
			return nil, errors.New("err: Cannot use option --kube-context and set attribute kubeContext.")
		}
		st.OverrideKubeContext = ld.overrideKubeContext
		// HelmDefaults.KubeContext is also overridden in here
		// to set default release value properly.
		st.HelmDefaults.KubeContext = ld.overrideKubeContext
	}

	if ld.namespace != "" {
		if st.OverrideNamespace != "" {
			return nil, errors.New("err: Cannot use option --namespace and set attribute namespace.")
		}
		st.OverrideNamespace = ld.namespace
	}

	if ld.chart != "" {
		if st.OverrideChart != "" {
			return nil, errors.New("err: Cannot use option --chart and set attribute chart.")
		}
		st.OverrideChart = ld.chart
	}

	return st, nil
}

func (ld *desiredStateLoader) loadFile(inheritedEnv *environment.Environment, baseDir, file string, evaluateBases bool) (*state.HelmState, error) {
	path, err := ld.remote.Locate(file)
	if err != nil {
		return nil, fmt.Errorf("locate: %v", err)
	}
	if file != path {
		ld.logger.Debugf("fetched remote \"%s\" to local cache \"%s\" and loading the latter...", file, path)
	}
	file = path
	return ld.loadFileWithOverrides(inheritedEnv, nil, baseDir, file, evaluateBases)
}

func (ld *desiredStateLoader) loadFileWithOverrides(inheritedEnv, overrodeEnv *environment.Environment, baseDir, file string, evaluateBases bool) (*state.HelmState, error) {
	var f string
	if filepath.IsAbs(file) {
		f = file
	} else {
		f = filepath.Join(baseDir, file)
	}

	fileBytes, err := ld.fs.ReadFile(f)
	if err != nil {
		return nil, err
	}

	self, err := ld.load(
		inheritedEnv,
		overrodeEnv,
		baseDir,
		f,
		fileBytes,
		evaluateBases,
	)

	if err != nil {
		return nil, err
	}

	for i, h := range self.Helmfiles {
		if h.Path == f {
			return nil, fmt.Errorf("%s contains a recursion into the same sub-helmfile at helmfiles[%d]", f, i)
		}
		if h.Path == "." {
			return nil, fmt.Errorf("%s contains a recursion into the the directory containing this helmfile at helmfiles[%d]", f, i)
		}
	}

	return self, nil
}

func (a *desiredStateLoader) underlying() *state.StateCreator {
	c := state.NewCreator(a.logger, a.fs, a.valsRuntime, a.getHelm, a.overrideHelmBinary, a.remote, a.enableLiveOutput, a.lockFilePath)
	c.LoadFile = a.loadFile
	return c
}

func (a *desiredStateLoader) rawLoad(yaml []byte, baseDir, file string, evaluateBases bool, env, overrodeEnv *environment.Environment) (*state.HelmState, error) {
	merged, err := env.Merge(overrodeEnv)
	if err != nil {
		return nil, err
	}

	st, err := a.underlying().ParseAndLoad(yaml, baseDir, file, a.env, evaluateBases, merged)
	if err != nil {
		return nil, err
	}

	helmfiles, err := st.ExpandedHelmfiles()
	if err != nil {
		return nil, err
	}
	st.Helmfiles = helmfiles

	return st, nil
}

func (ld *desiredStateLoader) load(env, overrodeEnv *environment.Environment, baseDir, filename string, content []byte, evaluateBases bool) (*state.HelmState, error) {
	// Allows part-splitting to work with CLRF-ed content
	normalizedContent := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	parts := bytes.Split(normalizedContent, []byte("\n---\n"))

	var finalState *state.HelmState

	for i, part := range parts {
		id := fmt.Sprintf("%s.part.%d", filename, i)

		var rawContent []byte

		if filepath.Ext(filename) == ".gotmpl" || !runtime.V1Mode {
			var yamlBuf *bytes.Buffer
			var err error

			if env == nil && overrodeEnv == nil {
				yamlBuf, err = ld.renderTemplatesToYaml(baseDir, id, part)
				if err != nil {
					return nil, fmt.Errorf("error during %s parsing: %v", id, err)
				}
			} else {
				yamlBuf, err = ld.renderTemplatesToYamlWithEnv(baseDir, id, part, env, overrodeEnv)
				if err != nil {
					return nil, fmt.Errorf("error during %s parsing: %v", id, err)
				}
			}
			rawContent = yamlBuf.Bytes()
		} else {
			rawContent = part
		}

		currentState, err := ld.rawLoad(
			rawContent,
			baseDir,
			filename,
			evaluateBases,
			env,
			overrodeEnv,
		)
		if err != nil {
			return nil, err
		}

		if finalState == nil {
			finalState = currentState
		} else {
			if err := mergo.Merge(&finalState.ReleaseSetSpec, &currentState.ReleaseSetSpec, mergo.WithOverride); err != nil {
				return nil, err
			}

			finalState.RenderedValues = currentState.RenderedValues
		}

		env = &finalState.Env

		ld.logger.Debugf("merged environment: %v", env)
	}

	return finalState, nil
}
