package app

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/state"
	"github.com/imdario/mergo"
	"github.com/variantdev/vals"
	"go.uber.org/zap"
)

const (
	DefaultHelmBinary = state.DefaultHelmBinary
)

type desiredStateLoader struct {
	overrideKubeContext string
	overrideHelmBinary  string

	env       string
	namespace string
	chart     string

	readFile          func(string) ([]byte, error)
	deleteFile        func(string) error
	fileExists        func(string) (bool, error)
	abs               func(string) (string, error)
	glob              func(string) ([]string, error)
	directoryExistsAt func(string) bool
	getHelm           func(*state.HelmState) helmexec.Interface

	remote      *remote.Remote
	logger      *zap.SugaredLogger
	valsRuntime vals.Evaluator
}

func (ld *desiredStateLoader) Load(f string, opts LoadOpts) (*state.HelmState, error) {
	var overrodeEnv *environment.Environment

	args := opts.Environment.OverrideValues

	if len(args) > 0 {
		if opts.CalleePath == "" {
			return nil, fmt.Errorf("bug: opts.CalleePath was nil: f=%s, opts=%v", f, opts)
		}
		storage := state.NewStorage(opts.CalleePath, ld.logger, ld.glob)
		envld := state.NewEnvironmentValuesLoader(storage, ld.readFile, ld.logger, ld.remote)
		handler := state.MissingFileHandlerError
		vals, err := envld.LoadEnvironmentValues(&handler, args, &environment.EmptyEnvironment)
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

	fileBytes, err := ld.readFile(f)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(f)

	var self *state.HelmState

	if !experimentalModeEnabled() || ext == ".gotmpl" {
		self, err = ld.renderAndLoad(
			inheritedEnv,
			overrodeEnv,
			baseDir,
			f,
			fileBytes,
			evaluateBases,
		)
	} else {
		self, err = ld.load(
			fileBytes,
			baseDir,
			file,
			evaluateBases,
			inheritedEnv,
			overrodeEnv,
		)
	}

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
	c := state.NewCreator(a.logger, a.readFile, a.fileExists, a.abs, a.glob, a.directoryExistsAt, a.valsRuntime, a.getHelm, a.overrideHelmBinary, a.remote)
	c.DeleteFile = a.deleteFile
	c.LoadFile = a.loadFile
	return c
}

func (a *desiredStateLoader) load(yaml []byte, baseDir, file string, evaluateBases bool, env, overrodeEnv *environment.Environment) (*state.HelmState, error) {
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

func (ld *desiredStateLoader) renderAndLoad(env, overrodeEnv *environment.Environment, baseDir, filename string, content []byte, evaluateBases bool) (*state.HelmState, error) {
	// Allows part-splitting to work with CLRF-ed content
	normalizedContent := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	parts := bytes.Split(normalizedContent, []byte("\n---\n"))

	var finalState *state.HelmState

	for i, part := range parts {
		var yamlBuf *bytes.Buffer
		var err error

		id := fmt.Sprintf("%s.part.%d", filename, i)

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

		currentState, err := ld.load(
			yamlBuf.Bytes(),
			baseDir,
			filename,
			evaluateBases,
			env,
			overrodeEnv,
		)
		if err != nil {
			return nil, err
		}

		for i, r := range currentState.Releases {
			if r.Chart == "" {
				return nil, fmt.Errorf("error during %s parsing: encountered empty chart while reading release %q at index %d", id, r.Name, i)
			}
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
