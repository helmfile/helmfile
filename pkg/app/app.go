package app

import (
	"bytes"
	goContext "context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/helmfile/vals"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/argparser"
	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/plugins"
	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/runtime"
	"github.com/helmfile/helmfile/pkg/state"
)

var CleanWaitGroup sync.WaitGroup
var Cancel goContext.CancelFunc

// App is the main application object.
type App struct {
	OverrideKubeContext        string
	OverrideHelmBinary         string
	OverrideKustomizeBinary    string
	EnableLiveOutput           bool
	StripArgsValuesOnExitError bool
	DisableForceUpdate         bool

	Logger      *zap.SugaredLogger
	Kubeconfig  string
	Env         string
	Namespace   string
	Chart       string
	Selectors   []string
	Args        string
	ValuesFiles []string
	Set         map[string]any

	FileOrDir string

	fs *filesystem.FileSystem

	remote *remote.Remote

	valsRuntime vals.Evaluator

	helms      map[helmKey]helmexec.Interface
	helmsMutex sync.Mutex

	ctx goContext.Context
}

type HelmRelease struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Enabled   bool   `json:"enabled"`
	Installed bool   `json:"installed"`
	Labels    string `json:"labels"`
	Chart     string `json:"chart"`
	Version   string `json:"version"`
}

func New(conf ConfigProvider) *App {
	ctx := goContext.Background()
	ctx, Cancel = goContext.WithCancel(ctx)

	return Init(&App{
		OverrideKubeContext:        conf.KubeContext(),
		OverrideHelmBinary:         conf.HelmBinary(),
		OverrideKustomizeBinary:    conf.KustomizeBinary(),
		EnableLiveOutput:           conf.EnableLiveOutput(),
		StripArgsValuesOnExitError: conf.StripArgsValuesOnExitError(),
		DisableForceUpdate:         conf.DisableForceUpdate(),
		Logger:                     conf.Logger(),
		Kubeconfig:                 conf.Kubeconfig(),
		Env:                        conf.Env(),
		Namespace:                  conf.Namespace(),
		Chart:                      conf.Chart(),
		Selectors:                  conf.Selectors(),
		Args:                       conf.Args(),
		FileOrDir:                  conf.FileOrDir(),
		ValuesFiles:                conf.StateValuesFiles(),
		Set:                        conf.StateValuesSet(),
		fs:                         filesystem.DefaultFileSystem(),
		ctx:                        ctx,
	})
}

func Init(app *App) *App {
	var err error
	app.valsRuntime, err = plugins.ValsInstance()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize vals runtime: %v", err))
	}

	if app.EnableLiveOutput {
		app.Logger.Info("Live output is enabled")
	}

	return app
}

func (a *App) Init(c InitConfigProvider) error {
	runner := &helmexec.ShellRunner{
		Logger:                     a.Logger,
		Ctx:                        a.ctx,
		StripArgsValuesOnExitError: a.StripArgsValuesOnExitError,
	}
	helmfileInit := NewHelmfileInit(a.OverrideHelmBinary, c, a.Logger, runner)
	return helmfileInit.Initialize()
}

func (a *App) Deps(c DepsConfigProvider) error {
	return a.ForEachState(func(run *Run) (_ bool, errs []error) {
		errs = run.Deps(c)
		return
	}, c.IncludeTransitiveNeeds(), SetFilter(true))
}

func (a *App) Repos(c ReposConfigProvider) error {
	return a.ForEachState(func(run *Run) (_ bool, errs []error) {
		reposErr := run.Repos(c)

		if reposErr != nil {
			errs = append(errs, reposErr)
		}

		return
	}, c.IncludeTransitiveNeeds(), SetFilter(true))
}

// TODO: Remove this function once Helmfile v0.x
func (a *App) DeprecatedSyncCharts(c DeprecatedChartsConfigProvider) error {
	return a.ForEachState(func(run *Run) (_ bool, errs []error) {
		err := run.withPreparedCharts("charts", state.ChartPrepareOptions{
			SkipRepos:   true,
			SkipDeps:    true,
			Concurrency: 2,
		}, func() {
			errs = run.DeprecatedSyncCharts(c)
		})

		if err != nil {
			errs = append(errs, err)
		}

		return
	}, c.IncludeTransitiveNeeds(), SetFilter(true))
}

func (a *App) Diff(c DiffConfigProvider) error {
	var allDiffDetectedErrs []error

	var affectedAny bool

	err := a.ForEachState(func(run *Run) (bool, []error) {
		var criticalErrs []error

		var msg *string

		var matched, affected bool

		var errs []error

		includeCRDs := !c.SkipCRDs()

		prepErr := run.withPreparedCharts("diff", state.ChartPrepareOptions{
			SkipRepos:              c.SkipRefresh() || c.SkipDeps(),
			SkipRefresh:            c.SkipRefresh(),
			SkipDeps:               c.SkipDeps(),
			IncludeCRDs:            &includeCRDs,
			Validate:               c.Validate(),
			Concurrency:            c.Concurrency(),
			IncludeTransitiveNeeds: c.IncludeNeeds(),
		}, func() {
			msg, matched, affected, errs = a.diff(run, c)
		})

		if msg != nil {
			a.Logger.Info(*msg)
		}

		if prepErr != nil {
			errs = append(errs, prepErr)
		}

		affectedAny = affectedAny || affected

		for i := range errs {
			switch e := errs[i].(type) {
			case *state.ReleaseError:
				switch e.Code {
				case 2:
					// See https://github.com/roboll/helmfile/issues/874
					allDiffDetectedErrs = append(allDiffDetectedErrs, e)
				default:
					criticalErrs = append(criticalErrs, e)
				}
			default:
				criticalErrs = append(criticalErrs, e)
			}
		}

		return matched, criticalErrs
	}, c.IncludeTransitiveNeeds())

	if err != nil {
		return err
	}

	if c.DetailedExitcode() && (len(allDiffDetectedErrs) > 0 || affectedAny) {
		// We take the first release error w/ exit status 2 (although all the defered errs should have exit status 2)
		// to just let helmfile itself to exit with 2
		// See https://github.com/roboll/helmfile/issues/749
		code := 2
		e := &Error{
			msg:  "Identified at least one change",
			code: &code,
		}
		return e
	}

	return nil
}

func (a *App) Template(c TemplateConfigProvider) error {
	return a.ForEachState(func(run *Run) (ok bool, errs []error) {
		includeCRDs := c.IncludeCRDs()

		// Live output should never be enabled for the "template" subcommand to avoid breaking `helmfile template | kubectl apply -f -`
		run.helm.SetEnableLiveOutput(false)

		// Reset helm extra args to not pollute BuildDeps() and AddRepo() on subsequent helmfiles
		// https://github.com/helmfile/helmfile/issues/1749
		run.helm.SetExtraArgs()

		prepErr := run.withPreparedCharts("template", state.ChartPrepareOptions{
			SkipRepos:              c.SkipRefresh() || c.SkipDeps(),
			SkipRefresh:            c.SkipRefresh(),
			SkipDeps:               c.SkipDeps(),
			IncludeCRDs:            &includeCRDs,
			SkipCleanup:            c.SkipCleanup(),
			Validate:               c.Validate(),
			Concurrency:            c.Concurrency(),
			IncludeTransitiveNeeds: c.IncludeNeeds(),
			Set:                    c.Set(),
			Values:                 c.Values(),
			KubeVersion:            c.KubeVersion(),
			TemplateArgs:           c.TemplateArgs(),
		}, func() {
			ok, errs = a.template(run, c)
		})

		if prepErr != nil {
			errs = append(errs, prepErr)
		}

		return
	}, c.IncludeTransitiveNeeds())
}

func (a *App) WriteValues(c WriteValuesConfigProvider) error {
	return a.ForEachState(func(run *Run) (ok bool, errs []error) {
		prepErr := run.withPreparedCharts("write-values", state.ChartPrepareOptions{
			SkipRepos:   c.SkipRefresh() || c.SkipDeps(),
			SkipRefresh: c.SkipRefresh(),
			SkipDeps:    c.SkipDeps(),
			SkipCleanup: c.SkipCleanup(),
			Concurrency: c.Concurrency(),
		}, func() {
			ok, errs = a.writeValues(run, c)
		})

		if prepErr != nil {
			errs = append(errs, prepErr)
		}

		return
	}, c.IncludeTransitiveNeeds(), SetFilter(true))
}

type MultiError struct {
	Errors []error
}

func (e *MultiError) Error() string {
	indent := func(text string, indent string) string {
		lines := strings.Split(text, "\n")

		var buf bytes.Buffer
		for _, l := range lines {
			buf.WriteString(indent)
			buf.WriteString(l)
			buf.WriteString("\n")
		}

		return buf.String()
	}

	lines := []string{fmt.Sprintf("Failed with %d errors:", len(e.Errors))}
	for i, err := range e.Errors {
		lines = append(lines, fmt.Sprintf("Error %d:\n\n%v", i+1, indent(err.Error(), "  ")))
	}

	return strings.Join(lines, "\n\n")
}

func (a *App) Lint(c LintConfigProvider) error {
	var deferredLintErrors []error

	err := a.ForEachState(func(run *Run) (ok bool, errs []error) {
		var lintErrs []error

		// `helm lint` on helm v2 and v3 does not support remote charts, that we need to set `forceDownload=true` here
		prepErr := run.withPreparedCharts("lint", state.ChartPrepareOptions{
			ForceDownload:          true,
			SkipRepos:              c.SkipRefresh() || c.SkipDeps(),
			SkipRefresh:            c.SkipRefresh(),
			SkipDeps:               c.SkipDeps(),
			SkipCleanup:            c.SkipCleanup(),
			Concurrency:            c.Concurrency(),
			IncludeTransitiveNeeds: c.IncludeNeeds(),
		}, func() {
			ok, lintErrs, errs = a.lint(run, c)
		})

		if prepErr != nil {
			errs = append(errs, prepErr)
		}

		if len(lintErrs) > 0 {
			deferredLintErrors = append(deferredLintErrors, lintErrs...)
		}

		return
	}, c.IncludeTransitiveNeeds())

	if err != nil {
		return err
	}

	if len(deferredLintErrors) > 0 {
		return &MultiError{Errors: deferredLintErrors}
	}

	return nil
}

func (a *App) Fetch(c FetchConfigProvider) error {
	return a.ForEachState(func(run *Run) (ok bool, errs []error) {
		prepErr := run.withPreparedCharts("pull", state.ChartPrepareOptions{
			ForceDownload:     true,
			SkipRefresh:       c.SkipRefresh(),
			SkipRepos:         c.SkipRefresh() || c.SkipDeps(),
			SkipDeps:          c.SkipDeps(),
			OutputDir:         c.OutputDir(),
			OutputDirTemplate: c.OutputDirTemplate(),
			Concurrency:       c.Concurrency(),
		}, func() {
		})

		if prepErr != nil {
			errs = append(errs, prepErr)
		}

		return
	}, false, SetFilter(true))
}

func (a *App) Sync(c SyncConfigProvider) error {
	return a.ForEachState(func(run *Run) (ok bool, errs []error) {
		includeCRDs := !c.SkipCRDs()

		prepErr := run.withPreparedCharts("sync", state.ChartPrepareOptions{
			SkipRepos:              c.SkipRefresh() || c.SkipDeps(),
			SkipRefresh:            c.SkipRefresh(),
			SkipDeps:               c.SkipDeps(),
			Wait:                   c.Wait(),
			WaitRetries:            c.WaitRetries(),
			WaitForJobs:            c.WaitForJobs(),
			IncludeCRDs:            &includeCRDs,
			IncludeTransitiveNeeds: c.IncludeNeeds(),
			Validate:               c.Validate(),
			Concurrency:            c.Concurrency(),
		}, func() {
			ok, errs = a.sync(run, c)
		})

		if prepErr != nil {
			errs = append(errs, prepErr)
		}

		return
	}, c.IncludeTransitiveNeeds())
}

func (a *App) Apply(c ApplyConfigProvider) error {
	var any bool

	mut := &sync.Mutex{}

	var opts []LoadOption

	opts = append(opts, SetRetainValuesFiles(c.RetainValuesFiles() || c.SkipCleanup()))

	err := a.ForEachState(func(run *Run) (ok bool, errs []error) {
		includeCRDs := !c.SkipCRDs()

		prepErr := run.withPreparedCharts("apply", state.ChartPrepareOptions{
			SkipRepos:              c.SkipRefresh() || c.SkipDeps(),
			SkipRefresh:            c.SkipRefresh(),
			SkipDeps:               c.SkipDeps(),
			Wait:                   c.Wait(),
			WaitRetries:            c.WaitRetries(),
			WaitForJobs:            c.WaitForJobs(),
			IncludeCRDs:            &includeCRDs,
			SkipCleanup:            c.RetainValuesFiles() || c.SkipCleanup(),
			Validate:               c.Validate(),
			Concurrency:            c.Concurrency(),
			IncludeTransitiveNeeds: c.IncludeNeeds(),
			TemplateArgs:           c.TemplateArgs(),
		}, func() {
			matched, updated, es := a.apply(run, c)

			mut.Lock()
			any = any || updated
			mut.Unlock()

			ok, errs = matched, es
		})

		if prepErr != nil {
			errs = append(errs, prepErr)
		}

		return
	}, c.IncludeTransitiveNeeds(), opts...)

	if err != nil {
		return err
	}

	if c.DetailedExitcode() && any {
		code := 2

		return &Error{msg: "", Errors: nil, code: &code}
	}

	return nil
}

func (a *App) Status(c StatusesConfigProvider) error {
	return a.ForEachState(func(run *Run) (ok bool, errs []error) {
		err := run.withPreparedCharts("status", state.ChartPrepareOptions{
			SkipRepos:   true,
			SkipDeps:    true,
			Concurrency: c.Concurrency(),
		}, func() {
			ok, errs = a.status(run, c)
		})

		if err != nil {
			errs = append(errs, err)
		}

		return
	}, false, SetFilter(true))
}

// TODO: Remove this function once Helmfile v0.x
func (a *App) Delete(c DeleteConfigProvider) error {
	return a.ForEachState(func(run *Run) (ok bool, errs []error) {
		if !c.SkipCharts() {
			err := run.withPreparedCharts("delete", state.ChartPrepareOptions{
				SkipRepos:     c.SkipRefresh() || c.SkipDeps(),
				SkipRefresh:   c.SkipRefresh(),
				SkipDeps:      c.SkipDeps(),
				Concurrency:   c.Concurrency(),
				DeleteWait:    c.DeleteWait(),
				DeleteTimeout: c.DeleteTimeout(),
			}, func() {
				ok, errs = a.delete(run, c.Purge(), c)
			})

			if err != nil {
				errs = append(errs, err)
			}
		} else {
			ok, errs = a.delete(run, c.Purge(), c)
		}
		return
	}, false, SetReverse(true))
}

func (a *App) Destroy(c DestroyConfigProvider) error {
	return a.ForEachState(func(run *Run) (ok bool, errs []error) {
		if !c.SkipCharts() {
			err := run.withPreparedCharts("destroy", state.ChartPrepareOptions{
				SkipRepos:     c.SkipRefresh() || c.SkipDeps(),
				SkipRefresh:   c.SkipRefresh(),
				SkipDeps:      c.SkipDeps(),
				Concurrency:   c.Concurrency(),
				DeleteWait:    c.DeleteWait(),
				DeleteTimeout: c.DeleteTimeout(),
			}, func() {
				ok, errs = a.delete(run, true, c)
			})
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			ok, errs = a.delete(run, true, c)
		}
		return
	}, false, SetReverse(true))
}

func (a *App) Test(c TestConfigProvider) error {
	return a.ForEachState(func(run *Run) (_ bool, errs []error) {
		if c.Cleanup() {
			a.Logger.Warnf("warn: requested cleanup will not be applied. " +
				"To clean up test resources with Helm 3, you have to remove them manually " +
				"or set helm.sh/hook-delete-policy\n")
		}

		err := run.withPreparedCharts("test", state.ChartPrepareOptions{
			SkipRepos:   c.SkipRefresh() || c.SkipDeps(),
			SkipRefresh: c.SkipRefresh(),
			SkipDeps:    c.SkipDeps(),
			Concurrency: c.Concurrency(),
		}, func() {
			errs = a.test(run, c)
		})

		if err != nil {
			errs = append(errs, err)
		}

		return
	}, false, SetFilter(true))
}

func (a *App) PrintDAGState(c DAGConfigProvider) error {
	var err error
	return a.ForEachState(func(run *Run) (ok bool, errs []error) {
		err = run.withPreparedCharts("show-dag", state.ChartPrepareOptions{
			SkipRepos:   true,
			SkipDeps:    true,
			Concurrency: 2,
		}, func() {
			err = a.dag(run)
			if err != nil {
				errs = append(errs, err)
			}
		})
		return ok, errs
	}, false, SetFilter(true))
}

func (a *App) PrintState(c StateConfigProvider) error {
	return a.ForEachState(func(run *Run) (_ bool, errs []error) {
		err := run.withPreparedCharts("build", state.ChartPrepareOptions{
			SkipRepos:   true,
			SkipDeps:    true,
			Concurrency: 2,
		}, func() {
			if c.EmbedValues() {
				for i := range run.state.Releases {
					r := run.state.Releases[i]

					values, err := run.state.LoadYAMLForEmbedding(&r, r.Values, r.MissingFileHandler, r.ValuesPathPrefix)
					if err != nil {
						errs = []error{err}
						return
					}

					run.state.Releases[i].Values = values

					secrets, err := run.state.LoadYAMLForEmbedding(&r, r.Secrets, r.MissingFileHandler, r.ValuesPathPrefix)
					if err != nil {
						errs = []error{err}
						return
					}

					run.state.Releases[i].Secrets = secrets
				}
			}

			stateYaml, err := run.state.ToYaml()
			if err != nil {
				errs = []error{err}
				return
			}

			sourceFile, err := run.state.FullFilePath()
			if err != nil {
				errs = []error{err}
				return
			}
			fmt.Printf("---\n#  Source: %s\n\n%+v", sourceFile, stateYaml)

			errs = []error{}
		})

		if err != nil {
			errs = append(errs, err)
		}

		return
	}, false, SetFilter(true))
}

func (a *App) dag(r *Run) error {
	st := r.state

	batches, err := st.PlanReleases(state.PlanOptions{SelectedReleases: st.Releases, Reverse: false, SkipNeeds: false, IncludeNeeds: true, IncludeTransitiveNeeds: true})
	if err != nil {
		return err
	}

	fmt.Print(printDAG(batches))

	return nil
}

func (a *App) ListReleases(c ListConfigProvider) error {
	var releases []*HelmRelease

	err := a.ForEachState(func(run *Run) (_ bool, errs []error) {
		var stateReleases []*HelmRelease
		var err error

		if !c.SkipCharts() {
			err = run.withPreparedCharts("list", state.ChartPrepareOptions{
				SkipRepos:   true,
				SkipDeps:    true,
				Concurrency: 2,
			}, func() {
				rel, err := a.list(run)
				if err != nil {
					panic(err)
				}
				stateReleases = rel
			})
		} else {
			stateReleases, err = a.list(run)
		}

		if err != nil {
			errs = append(errs, err)
		}

		releases = append(releases, stateReleases...)

		return
	}, false, SetFilter(true))

	if err != nil {
		return err
	}

	if c.Output() == "json" {
		err = FormatAsJson(releases)
	} else {
		err = FormatAsTable(releases)
	}

	return err
}

func (a *App) list(run *Run) ([]*HelmRelease, error) {
	var releases []*HelmRelease

	for _, r := range run.state.Releases {
		labels := ""
		if r.Labels == nil {
			r.Labels = map[string]string{}
		}
		for k, v := range run.state.CommonLabels {
			r.Labels[k] = v
		}

		var keys []string
		for k := range r.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := r.Labels[k]
			labels = fmt.Sprintf("%s,%s:%s", labels, k, v)
		}
		labels = strings.Trim(labels, ",")

		enabled, err := state.ConditionEnabled(r, run.state.Values())
		if err != nil {
			return nil, err
		}

		releases = append(releases, &HelmRelease{
			Name:      r.Name,
			Namespace: r.Namespace,
			Installed: r.Desired(),
			Enabled:   enabled,
			Labels:    labels,
			Chart:     r.Chart,
			Version:   r.Version,
		})
	}

	return releases, nil
}

func (a *App) within(dir string, do func() error) error {
	if dir == "." {
		return do()
	}

	prev, err := a.fs.Getwd()
	if err != nil {
		return fmt.Errorf("failed getting current working direcotyr: %v", err)
	}

	absDir, err := a.fs.Abs(dir)
	if err != nil {
		return err
	}

	a.Logger.Debugf("changing working directory to \"%s\"", absDir)

	if err := a.fs.Chdir(absDir); err != nil {
		return fmt.Errorf("failed changing working directory to \"%s\": %v", absDir, err)
	}

	appErr := do()

	a.Logger.Debugf("changing working directory back to \"%s\"", prev)

	if chdirBackErr := a.fs.Chdir(prev); chdirBackErr != nil {
		if appErr != nil {
			a.Logger.Warnf("%v", appErr)
		}
		return fmt.Errorf("failed chaging working directory back to \"%s\": %v", prev, chdirBackErr)
	}

	return appErr
}

func (a *App) visitStateFiles(fileOrDir string, opts LoadOpts, do func(string, string) error) error {
	desiredStateFiles, err := a.findDesiredStateFiles(fileOrDir, opts)
	if err != nil {
		return appError("", err)
	}

	for _, relPath := range desiredStateFiles {
		var file string
		var dir string
		if a.fs.DirectoryExistsAt(relPath) {
			file = relPath
			dir = relPath
		} else {
			file = filepath.Base(relPath)
			dir = filepath.Dir(relPath)
		}

		a.Logger.Debugf("processing file \"%s\" in directory \"%s\"", file, dir)

		absd, errAbsDir := a.fs.Abs(dir)
		if errAbsDir != nil {
			return errAbsDir
		}
		err := a.within(absd, func() error {
			return do(file, absd)
		})
		if err != nil {
			return appError(fmt.Sprintf("in %s/%s", dir, file), err)
		}
	}

	return nil
}

func (a *App) loadDesiredStateFromYaml(file string, opts ...LoadOpts) (*state.HelmState, error) {
	var op LoadOpts
	if len(opts) > 0 {
		op = opts[0]
	}

	ld := &desiredStateLoader{
		fs:        a.fs,
		env:       a.Env,
		namespace: a.Namespace,
		chart:     a.Chart,
		logger:    a.Logger,
		remote:    a.remote,

		overrideKubeContext:     a.OverrideKubeContext,
		overrideHelmBinary:      a.OverrideHelmBinary,
		overrideKustomizeBinary: a.OverrideKustomizeBinary,
		enableLiveOutput:        a.EnableLiveOutput,
		getHelm:                 a.getHelm,
		valsRuntime:             a.valsRuntime,
	}

	return ld.Load(file, op)
}

type helmKey struct {
	Binary  string
	Context string
}

func createHelmKey(bin, kubectx string) helmKey {
	return helmKey{
		Binary:  bin,
		Context: kubectx,
	}
}

// GetHelm returns the global helm exec instance for the specified state that is used for helmfile-wise operation
// like decrypting environment secrets.
//
// This is currently used for running all the helm commands for reconciling releases. But this may change in the future
// once we enable each release to have its own helm binary/version.
func (a *App) getHelm(st *state.HelmState) helmexec.Interface {
	a.helmsMutex.Lock()
	defer a.helmsMutex.Unlock()

	if a.helms == nil {
		a.helms = map[helmKey]helmexec.Interface{}
	}

	bin := st.DefaultHelmBinary
	kubeconfig := a.Kubeconfig
	kubectx := st.HelmDefaults.KubeContext

	key := createHelmKey(bin, kubectx)

	if _, ok := a.helms[key]; !ok {
		a.helms[key] = helmexec.New(bin, helmexec.HelmExecOptions{EnableLiveOutput: a.EnableLiveOutput, DisableForceUpdate: a.DisableForceUpdate}, a.Logger, kubeconfig, kubectx, &helmexec.ShellRunner{
			Logger:                     a.Logger,
			Ctx:                        a.ctx,
			StripArgsValuesOnExitError: a.StripArgsValuesOnExitError,
		})
	}

	return a.helms[key]
}

func (a *App) visitStates(fileOrDir string, defOpts LoadOpts, converge func(*state.HelmState) (bool, []error)) error {
	noMatchInHelmfiles := true

	err := a.visitStateFiles(fileOrDir, defOpts, func(f, d string) (retErr error) {
		opts := defOpts.DeepCopy()

		if opts.CalleePath == "" {
			opts.CalleePath = f
		}

		st, err := a.loadDesiredStateFromYaml(f, opts)

		ctx := context{app: a, st: st, retainValues: defOpts.RetainValuesFiles}

		if err != nil {
			switch stateLoadErr := err.(type) {
			// Addresses https://github.com/roboll/helmfile/issues/279
			case *state.StateLoadError:
				switch stateLoadErr.Cause.(type) {
				case *state.UndefinedEnvError:
					return nil
				default:
					return ctx.wrapErrs(err)
				}
			default:
				return ctx.wrapErrs(err)
			}
		}
		st.Selectors = opts.Selectors

		visitSubHelmfiles := func() error {
			if len(st.Helmfiles) > 0 {
				noMatchInSubHelmfiles := true
				for i, m := range st.Helmfiles {
					optsForNestedState := LoadOpts{
						CalleePath:        filepath.Join(d, f),
						Environment:       m.Environment,
						Reverse:           defOpts.Reverse,
						RetainValuesFiles: defOpts.RetainValuesFiles,
					}
					// assign parent selector to sub helm selector in legacy mode or do not inherit in experimental mode
					if (m.Selectors == nil && !isExplicitSelectorInheritanceEnabled()) || m.SelectorsInherited {
						optsForNestedState.Selectors = opts.Selectors
					} else {
						optsForNestedState.Selectors = m.Selectors
					}

					if err := a.visitStates(m.Path, optsForNestedState, converge); err != nil {
						switch err.(type) {
						case *NoMatchingHelmfileError:

						default:
							return appError(fmt.Sprintf("in .helmfiles[%d]", i), err)
						}
					} else {
						noMatchInSubHelmfiles = false
					}
				}
				noMatchInHelmfiles = noMatchInHelmfiles && noMatchInSubHelmfiles
			}
			return nil
		}

		if !opts.Reverse {
			err = visitSubHelmfiles()
			if err != nil {
				return err
			}
		}

		templated, tmplErr := st.ExecuteTemplates()
		if tmplErr != nil {
			return appError(fmt.Sprintf("failed executing release templates in \"%s\"", f), tmplErr)
		}

		var (
			processed bool
			errs      []error
		)

		// Ensure every temporary files and directories generated while running
		// the converge function is clean up before exiting this function in all the three cases below:
		// - This function returned nil
		// - This function returned an err
		// - Helmfile received SIGINT or SIGTERM while running this function
		// For the last case you also need a signal handler in main.go.
		// Ideally though, this CleanWaitGroup should gone and be replaced by a context cancellation propagation.
		// See https://github.com/helmfile/helmfile/pull/418 for more details.
		CleanWaitGroup.Add(1)
		defer func() {
			defer CleanWaitGroup.Done()
			cleanErr := context{app: a, st: templated, retainValues: defOpts.RetainValuesFiles}.clean(errs)
			if retErr == nil {
				retErr = cleanErr
			} else if cleanErr != nil {
				a.Logger.Debugf("Failed to clean up temporary files generated while processing %q: %v", templated.FilePath, cleanErr)
			}
		}()

		processed, errs = converge(templated)

		noMatchInHelmfiles = noMatchInHelmfiles && !processed

		if opts.Reverse {
			err = visitSubHelmfiles()
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	if noMatchInHelmfiles {
		return &NoMatchingHelmfileError{selectors: a.Selectors, env: a.Env}
	}

	return nil
}

type LoadOption func(o *LoadOpts)

var (
	SetReverse = func(r bool) func(o *LoadOpts) {
		return func(o *LoadOpts) {
			o.Reverse = r
		}
	}

	SetRetainValuesFiles = func(r bool) func(o *LoadOpts) {
		return func(o *LoadOpts) {
			o.RetainValuesFiles = r
		}
	}

	SetFilter = func(f bool) func(o *LoadOpts) {
		return func(o *LoadOpts) {
			o.Filter = f
		}
	}
)

func (a *App) ForEachState(do func(*Run) (bool, []error), includeTransitiveNeeds bool, o ...LoadOption) error {
	ctx := NewContext()
	err := a.visitStatesWithSelectorsAndRemoteSupport(a.FileOrDir, func(st *state.HelmState) (bool, []error) {
		helm := a.getHelm(st)

		run, err := NewRun(st, helm, ctx)
		if err != nil {
			return false, []error{err}
		}
		return do(run)
	}, includeTransitiveNeeds, o...)

	return err
}

func printBatches(batches [][]state.Release) string {
	buf := &bytes.Buffer{}

	w := new(tabwriter.Writer)

	w.Init(buf, 0, 1, 1, ' ', 0)

	_, _ = fmt.Fprintln(w, "GROUP\tRELEASES")

	for i, batch := range batches {
		ids := []string{}
		for _, r := range batch {
			ids = append(ids, state.ReleaseToID(&r.ReleaseSpec))
		}
		_, _ = fmt.Fprintf(w, "%d\t%s\n", i+1, strings.Join(ids, ", "))
	}

	_ = w.Flush()

	return buf.String()
}

func printDAG(batches [][]state.Release) string {
	buf := &bytes.Buffer{}

	w := new(tabwriter.Writer)

	w.Init(buf, 0, 1, 1, ' ', 0)

	_, _ = fmt.Fprintln(w, "GROUP\tRELEASE\tDEPENDENCIES")

	for i, batch := range batches {
		for _, r := range batch {
			id := state.ReleaseToID(&r.ReleaseSpec)
			needs := r.ReleaseSpec.Needs
			_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", i+1, id, strings.Join(needs, ", "))
		}
	}

	_ = w.Flush()

	return buf.String()
}

// nolint: unparam
func withDAG(templated *state.HelmState, helm helmexec.Interface, logger *zap.SugaredLogger, opts state.PlanOptions, converge func(*state.HelmState, helmexec.Interface) (bool, []error)) (bool, []error) {
	batches, err := templated.PlanReleases(opts)
	if err != nil {
		return false, []error{err}
	}

	return withBatches(opts.Purpose, templated, batches, helm, logger, converge)
}

func withBatches(purpose string, templated *state.HelmState, batches [][]state.Release, helm helmexec.Interface, logger *zap.SugaredLogger, converge func(*state.HelmState, helmexec.Interface) (bool, []error)) (bool, []error) {
	numBatches := len(batches)

	if purpose == "" {
		purpose = "processing"
	}

	logger.Debugf("%s %d groups of releases in this order:\n%s", purpose, numBatches, printBatches(batches))

	any := false

	for i, batch := range batches {
		var targets []state.ReleaseSpec

		for _, marked := range batch {
			targets = append(targets, marked.ReleaseSpec)
		}

		var releaseIds []string
		for _, r := range targets {
			release := r
			releaseIds = append(releaseIds, state.ReleaseToID(&release))
		}

		logger.Debugf("%s releases in group %d/%d: %s", purpose, i+1, numBatches, strings.Join(releaseIds, ", "))

		batchSt := *templated
		batchSt.Releases = targets

		processed, errs := converge(&batchSt, helm)

		if len(errs) > 0 {
			return false, errs
		}

		any = any || processed
	}

	return any, nil
}

type Opts struct {
	DAGEnabled bool
}

func (a *App) visitStatesWithSelectorsAndRemoteSupport(fileOrDir string, converge func(*state.HelmState) (bool, []error), includeTransitiveNeeds bool, opt ...LoadOption) error {
	opts := LoadOpts{
		Selectors: a.Selectors,
	}

	for _, o := range opt {
		o(&opts)
	}

	envvals := []any{}

	for _, v := range a.ValuesFiles {
		envvals = append(envvals, v)
	}

	if len(a.Set) > 0 {
		envvals = append(envvals, a.Set)
	}

	if len(envvals) > 0 {
		opts.Environment.OverrideValues = envvals
	}

	a.remote = remote.NewRemote(a.Logger, "", a.fs)

	f := converge
	if opts.Filter {
		f = func(st *state.HelmState) (bool, []error) {
			return processFilteredReleases(st, func(st *state.HelmState) []error {
				_, err := converge(st)
				return err
			},
				includeTransitiveNeeds)
		}
	}

	// pre-overrides HelmState
	fHelmStatsWithOverrides := func(st *state.HelmState) (bool, []error) {
		var err error
		st.Releases, err = st.GetReleasesWithOverrides()
		if err != nil {
			return false, []error{err}
		}
		return f(st)
	}

	return a.visitStates(fileOrDir, opts, fHelmStatsWithOverrides)
}

func processFilteredReleases(st *state.HelmState, converge func(st *state.HelmState) []error, includeTransitiveNeeds bool) (bool, []error) {
	if len(st.Selectors) > 0 {
		err := st.FilterReleases(includeTransitiveNeeds)
		if err != nil {
			return false, []error{err}
		}
	}

	if err := checkDuplicates(st.Releases); err != nil {
		return false, []error{err}
	}

	errs := converge(st)

	processed := len(st.Releases) != 0 && len(errs) == 0

	return processed, errs
}

func checkDuplicates(releases []state.ReleaseSpec) error {
	type Key struct {
		Namespace, Name, KubeContext string
	}

	releaseNameCounts := map[Key]int{}
	for _, r := range releases {
		namespace := r.Namespace
		releaseNameCounts[Key{namespace, r.Name, r.KubeContext}]++
	}
	for name, c := range releaseNameCounts {
		if c > 1 {
			var msg string

			if name.Namespace != "" {
				msg += fmt.Sprintf(" in namespace %q", name.Namespace)
			}

			if name.KubeContext != "" {
				msg += fmt.Sprintf(" in kubecontext %q", name.KubeContext)
			}

			return fmt.Errorf("duplicate release %q found%s: there were %d releases named \"%s\" matching specified selector", name.Name, msg, c, name.Name)
		}
	}

	return nil
}

func (a *App) Wrap(converge func(*state.HelmState, helmexec.Interface) []error) func(st *state.HelmState, helm helmexec.Interface, includeTransitiveNeeds bool) (bool, []error) {
	return func(st *state.HelmState, helm helmexec.Interface, includeTransitiveNeeds bool) (bool, []error) {
		return processFilteredReleases(st, func(st *state.HelmState) []error {
			return converge(st, helm)
		}, includeTransitiveNeeds)
	}
}

func (a *App) WrapWithoutSelector(converge func(*state.HelmState, helmexec.Interface) []error) func(st *state.HelmState, helm helmexec.Interface) (bool, []error) {
	return func(st *state.HelmState, helm helmexec.Interface) (bool, []error) {
		errs := converge(st, helm)
		processed := len(st.Releases) != 0 && len(errs) == 0
		return processed, errs
	}
}

func (a *App) findDesiredStateFiles(specifiedPath string, opts LoadOpts) ([]string, error) {
	path, err := a.remote.Locate(specifiedPath, "states")
	if err != nil {
		return nil, fmt.Errorf("locate: %v", err)
	}
	if specifiedPath != path {
		a.Logger.Debugf("fetched remote \"%s\" to local cache \"%s\" and loading the latter...", specifiedPath, path)
	}
	specifiedPath = path

	var helmfileDir string
	if specifiedPath != "" {
		switch {
		case a.fs.FileExistsAt(specifiedPath):
			return []string{specifiedPath}, nil
		case a.fs.DirectoryExistsAt(specifiedPath):
			helmfileDir = specifiedPath
		default:
			return []string{}, fmt.Errorf("specified state file %s is not found", specifiedPath)
		}
	} else {
		var defaultFile string
		DefaultGotmplHelmfile := DefaultHelmfile + ".gotmpl"
		if a.fs.FileExistsAt(DefaultHelmfile) && a.fs.FileExistsAt(DefaultGotmplHelmfile) {
			return []string{}, fmt.Errorf("both %s and %s.gotmpl exist. Please remove one of them", DefaultHelmfile, DefaultHelmfile)
		}
		switch {
		case a.fs.FileExistsAt(DefaultHelmfile):
			defaultFile = DefaultHelmfile

		case a.fs.FileExistsAt(DefaultGotmplHelmfile):
			defaultFile = DefaultGotmplHelmfile

		// TODO: Remove this block when we remove v0 code
		case !runtime.V1Mode && a.fs.FileExistsAt(DeprecatedHelmfile):
			a.Logger.Warnf(
				"warn: %s is being loaded: %s is deprecated in favor of %s. See https://github.com/roboll/helmfile/issues/25 for more information",
				DeprecatedHelmfile,
				DeprecatedHelmfile,
				DefaultHelmfile,
			)
			defaultFile = DeprecatedHelmfile
		}

		switch {
		case a.fs.DirectoryExistsAt(DefaultHelmfileDirectory):
			if defaultFile != "" {
				return []string{}, fmt.Errorf("configuration conlict error: you can have either %s or %s, but not both", defaultFile, DefaultHelmfileDirectory)
			}

			helmfileDir = DefaultHelmfileDirectory
		case defaultFile != "":
			return []string{defaultFile}, nil
		default:
			return []string{}, fmt.Errorf("no state file found. It must be named %s/*.{yaml,yml,yaml.gotmpl,yml.gotmpl}, %s, or %s, otherwise specified with the --file flag or %s environment variable", DefaultHelmfileDirectory, DefaultHelmfile, DefaultGotmplHelmfile, envvar.FilePath)
		}
	}

	files := []string{}

	ymlFiles, err := a.fs.Glob(filepath.Join(helmfileDir, "*.y*ml"))
	if err != nil {
		return []string{}, err
	}
	gotmplFiles, err := a.fs.Glob(filepath.Join(helmfileDir, "*.y*ml.gotmpl"))
	if err != nil {
		return []string{}, err
	}

	files = append(files, ymlFiles...)
	files = append(files, gotmplFiles...)

	if opts.Reverse {
		sort.Slice(files, func(i, j int) bool {
			return files[j] < files[i]
		})
	} else {
		sort.Slice(files, func(i, j int) bool {
			return files[i] < files[j]
		})
	}

	a.Logger.Debugf("found %d helmfile state files in %s: %s", len(ymlFiles)+len(gotmplFiles), helmfileDir, strings.Join(files, ", "))

	return files, nil
}

func (a *App) getSelectedReleases(r *Run, includeTransitiveNeeds bool) ([]state.ReleaseSpec, []state.ReleaseSpec, error) {
	selected, err := r.state.GetSelectedReleases(includeTransitiveNeeds)
	if err != nil {
		return nil, nil, err
	}

	selectedIds := map[string]state.ReleaseSpec{}
	selectedCounts := map[string]int{}
	for _, r := range selected {
		id := state.ReleaseToID(&r)
		selectedIds[id] = r
		selectedCounts[id]++

		if dupCount := selectedCounts[id]; dupCount > 1 {
			return nil, nil, fmt.Errorf("found %d duplicate releases with ID %q", dupCount, id)
		}
	}

	allReleases := r.state.Releases

	groupsByID := map[string][]*state.ReleaseSpec{}
	for _, r := range allReleases {
		groupsByID[state.ReleaseToID(&r)] = append(groupsByID[state.ReleaseToID(&r)], &r)
	}

	var deduplicated []state.ReleaseSpec

	dedupedBefore := map[string]struct{}{}

	// We iterate over allReleases rather than groupsByID
	// to preserve the order of releases
	for _, seq := range allReleases {
		release := seq
		id := state.ReleaseToID(&release)

		rs := groupsByID[id]

		if len(rs) == 1 {
			deduplicated = append(deduplicated, *rs[0])
			continue
		}

		if _, ok := dedupedBefore[id]; ok {
			continue
		}

		// We keep the selected one only when there were two or more duplicate
		// releases in the helmfile config.
		// Otherwise we can't compute the DAG of releases correctly.
		r, deduped := selectedIds[id]
		if deduped {
			deduplicated = append(deduplicated, r)
			dedupedBefore[id] = struct{}{}
		}
	}

	if err := checkDuplicates(deduplicated); err != nil {
		return nil, nil, err
	}

	var extra string

	if len(r.state.Selectors) > 0 {
		extra = " matching " + strings.Join(r.state.Selectors, ",")
	}

	a.Logger.Debugf("%d release(s)%s found in %s\n", len(selected), extra, r.state.FilePath)

	return selected, deduplicated, nil
}

func (a *App) apply(r *Run, c ApplyConfigProvider) (bool, bool, []error) {
	st := r.state
	helm := r.helm

	helm.SetExtraArgs(GetArgs(c.Args(), r.state)...)

	selectedReleases, selectedAndNeededReleases, err := a.getSelectedReleases(r, c.IncludeTransitiveNeeds())
	if err != nil {
		return false, false, []error{err}
	}
	if len(selectedReleases) == 0 {
		return false, false, nil
	}

	// This is required when you're trying to deduplicate releases by the selector.
	// Without this, `PlanReleases` conflates duplicates and return both in `batches`,
	// even if we provided `SelectedReleases: selectedReleases`.
	// See https://github.com/roboll/helmfile/issues/1818 for more context.
	st.Releases = selectedAndNeededReleases

	plan, err := st.PlanReleases(state.PlanOptions{Reverse: false, SelectedReleases: selectedReleases, SkipNeeds: c.SkipNeeds(), IncludeNeeds: c.IncludeNeeds(), IncludeTransitiveNeeds: c.IncludeTransitiveNeeds()})
	if err != nil {
		return false, false, []error{err}
	}

	var toApplyWithNeeds []state.ReleaseSpec

	for _, rs := range plan {
		for _, r := range rs {
			toApplyWithNeeds = append(toApplyWithNeeds, r.ReleaseSpec)
		}
	}

	// Do build deps and prepare only on selected releases so that we won't waste time
	// on running various helm commands on unnecessary releases
	st.Releases = toApplyWithNeeds

	// helm must be 2.11+ and helm-diff should be provided `--detailed-exitcode` in order for `helmfile apply` to work properly
	detailedExitCode := true

	diffOpts := &state.DiffOpts{
		Color:                   c.Color(),
		NoColor:                 c.NoColor(),
		Context:                 c.Context(),
		Output:                  c.DiffOutput(),
		Set:                     c.Set(),
		SkipCleanup:             c.RetainValuesFiles() || c.SkipCleanup(),
		SkipDiffOnInstall:       c.SkipDiffOnInstall(),
		ReuseValues:             c.ReuseValues(),
		ResetValues:             c.ResetValues(),
		DiffArgs:                c.DiffArgs(),
		PostRenderer:            c.PostRenderer(),
		PostRendererArgs:        c.PostRendererArgs(),
		SkipSchemaValidation:    c.SkipSchemaValidation(),
		SuppressOutputLineRegex: c.SuppressOutputLineRegex(),
	}

	infoMsg, releasesToBeUpdated, releasesToBeDeleted, errs := r.diff(false, detailedExitCode, c, diffOpts)
	if len(errs) > 0 {
		return false, false, errs
	}

	var toDelete []state.ReleaseSpec
	for _, r := range releasesToBeDeleted {
		toDelete = append(toDelete, r)
	}

	var toUpdate []state.ReleaseSpec
	for _, r := range releasesToBeUpdated {
		toUpdate = append(toUpdate, r)
	}

	releasesWithNoChange := map[string]state.ReleaseSpec{}
	for _, r := range toApplyWithNeeds {
		release := r
		id := state.ReleaseToID(&release)
		_, uninstalled := releasesToBeDeleted[id]
		_, updated := releasesToBeUpdated[id]
		if !uninstalled && !updated {
			releasesWithNoChange[id] = release
		}
	}

	infoMsgStr := ""
	if infoMsg != nil {
		infoMsgStr = *infoMsg
	}

	confMsg := fmt.Sprintf(`%s
Do you really want to apply?
  Helmfile will apply all your changes, as shown above.

`, infoMsgStr)

	interactive := c.Interactive()
	if !interactive && infoMsgStr != "" {
		a.Logger.Debug(infoMsgStr)
	}

	var applyErrs []error

	affectedReleases := state.AffectedReleases{}

	// Traverse DAG of all the releases so that we don't suffer from false-positive missing dependencies
	st.Releases = selectedAndNeededReleases

	if !interactive || interactive && r.askForConfirmation(confMsg) {
		if _, preapplyErrors := withDAG(st, helm, a.Logger, state.PlanOptions{Purpose: "invoking preapply hooks for", Reverse: true, SelectedReleases: toApplyWithNeeds, SkipNeeds: true}, a.WrapWithoutSelector(func(subst *state.HelmState, helm helmexec.Interface) []error {
			for _, r := range subst.Releases {
				release := r
				if _, err := st.TriggerPreapplyEvent(&release, "apply"); err != nil {
					return []error{err}
				}
			}

			return nil
		})); len(preapplyErrors) > 0 {
			return true, false, preapplyErrors
		}

		// We deleted releases by traversing the DAG in reverse order
		if len(releasesToBeDeleted) > 0 {
			_, deletionErrs := withDAG(st, helm, a.Logger, state.PlanOptions{Reverse: true, SelectedReleases: toDelete, SkipNeeds: true}, a.WrapWithoutSelector(func(subst *state.HelmState, helm helmexec.Interface) []error {
				var rs []state.ReleaseSpec

				for _, r := range subst.Releases {
					release := r
					if r2, ok := releasesToBeDeleted[state.ReleaseToID(&release)]; ok {
						rs = append(rs, r2)
					}
				}

				subst.Releases = rs

				return subst.DeleteReleasesForSync(&affectedReleases, helm, c.Concurrency(), c.Cascade())
			}))

			if len(deletionErrs) > 0 {
				applyErrs = append(applyErrs, deletionErrs...)
			}
		}

		// We upgrade releases by traversing the DAG
		if len(releasesToBeUpdated) > 0 {
			_, updateErrs := withDAG(st, helm, a.Logger, state.PlanOptions{SelectedReleases: toUpdate, Reverse: false, SkipNeeds: true, IncludeTransitiveNeeds: c.IncludeTransitiveNeeds()}, a.WrapWithoutSelector(func(subst *state.HelmState, helm helmexec.Interface) []error {
				var rs []state.ReleaseSpec

				for _, r := range subst.Releases {
					release := r
					if r2, ok := releasesToBeUpdated[state.ReleaseToID(&release)]; ok {
						rs = append(rs, r2)
					}
				}

				subst.Releases = rs

				syncOpts := &state.SyncOpts{
					Set:                  c.Set(),
					SkipCleanup:          c.RetainValuesFiles() || c.SkipCleanup(),
					SkipCRDs:             c.SkipCRDs(),
					Wait:                 c.Wait(),
					WaitRetries:          c.WaitRetries(),
					WaitForJobs:          c.WaitForJobs(),
					ReuseValues:          c.ReuseValues(),
					ResetValues:          c.ResetValues(),
					PostRenderer:         c.PostRenderer(),
					PostRendererArgs:     c.PostRendererArgs(),
					SkipSchemaValidation: c.SkipSchemaValidation(),
					SyncArgs:             c.SyncArgs(),
					HideNotes:            c.HideNotes(),
					TakeOwnership:        c.TakeOwnership(),
				}
				return subst.SyncReleases(&affectedReleases, helm, c.Values(), c.Concurrency(), syncOpts)
			}))

			if len(updateErrs) > 0 {
				applyErrs = append(applyErrs, updateErrs...)
			}
		}
	}

	affectedReleases.DisplayAffectedReleases(c.Logger())

	for id := range releasesWithNoChange {
		r := releasesWithNoChange[id]
		if _, err := st.TriggerCleanupEvent(&r, "apply"); err != nil {
			a.Logger.Warnf("warn: %v\n", err)
		}
	}
	if releasesToBeDeleted == nil && releasesToBeUpdated == nil {
		return true, false, nil
	}

	return true, true, applyErrs
}

func (a *App) delete(r *Run, purge bool, c DestroyConfigProvider) (bool, []error) {
	st := r.state
	helm := r.helm

	affectedReleases := state.AffectedReleases{}

	toSync, _, err := a.getSelectedReleases(r, false)
	if err != nil {
		return false, []error{err}
	}
	if len(toSync) == 0 {
		return false, nil
	}

	toDelete, err := st.DetectReleasesToBeDeleted(helm, toSync)
	if err != nil {
		return false, []error{err}
	}

	releasesToDelete := map[string]state.ReleaseSpec{}
	for _, r := range toDelete {
		release := r
		id := state.ReleaseToID(&release)
		releasesToDelete[id] = release
	}

	releasesWithNoChange := map[string]state.ReleaseSpec{}
	for _, r := range toSync {
		release := r
		id := state.ReleaseToID(&release)
		_, uninstalled := releasesToDelete[id]
		if !uninstalled {
			releasesWithNoChange[id] = release
		}
	}

	for id := range releasesWithNoChange {
		r := releasesWithNoChange[id]
		if _, err := st.TriggerCleanupEvent(&r, "delete"); err != nil {
			a.Logger.Warnf("warn: %v\n", err)
		}
	}

	names := make([]string, len(toSync))
	for i, r := range toSync {
		names[i] = fmt.Sprintf("  %s (%s)", r.Name, r.Chart)
	}

	var errs []error

	msg := fmt.Sprintf(`Affected releases are:
%s

Do you really want to delete?
  Helmfile will delete all your releases, as shown above.

`, strings.Join(names, "\n"))
	interactive := c.Interactive()
	if !interactive || interactive && r.askForConfirmation(msg) {
		r.helm.SetExtraArgs(GetArgs(c.Args(), r.state)...)

		if len(releasesToDelete) > 0 {
			_, deletionErrs := withDAG(st, helm, a.Logger, state.PlanOptions{SelectedReleases: toDelete, Reverse: true, SkipNeeds: true}, a.WrapWithoutSelector(func(subst *state.HelmState, helm helmexec.Interface) []error {
				return subst.DeleteReleases(&affectedReleases, helm, c.Concurrency(), purge, c.Cascade())
			}))

			if len(deletionErrs) > 0 {
				errs = append(errs, deletionErrs...)
			}
		}
	}
	affectedReleases.DisplayAffectedReleases(c.Logger())
	return true, errs
}

func (a *App) diff(r *Run, c DiffConfigProvider) (*string, bool, bool, []error) {
	var (
		infoMsg          *string
		updated, deleted map[string]state.ReleaseSpec
	)

	ok, errs := a.withNeeds(r, c, true, func(st *state.HelmState) []error {
		helm := r.helm

		var errs []error

		helm.SetExtraArgs(GetArgs(c.Args(), r.state)...)

		opts := &state.DiffOpts{
			Context:                 c.Context(),
			Output:                  c.DiffOutput(),
			Color:                   c.Color(),
			NoColor:                 c.NoColor(),
			Set:                     c.Set(),
			DiffArgs:                c.DiffArgs(),
			SkipDiffOnInstall:       c.SkipDiffOnInstall(),
			ReuseValues:             c.ReuseValues(),
			ResetValues:             c.ResetValues(),
			PostRenderer:            c.PostRenderer(),
			PostRendererArgs:        c.PostRendererArgs(),
			SkipSchemaValidation:    c.SkipSchemaValidation(),
			SuppressOutputLineRegex: c.SuppressOutputLineRegex(),
		}

		filtered := &Run{
			state: st,
			helm:  helm,
			ctx:   r.ctx,
			Ask:   r.Ask,
		}
		infoMsg, updated, deleted, errs = filtered.diff(true, c.DetailedExitcode(), c, opts)

		return errs
	})

	return infoMsg, ok, len(deleted) > 0 || len(updated) > 0, errs
}

func (a *App) lint(r *Run, c LintConfigProvider) (bool, []error, []error) {
	var deferredLintErrs []error

	ok, errs := a.withNeeds(r, c, false, func(st *state.HelmState) []error {
		helm := r.helm

		args := GetArgs(c.Args(), st)

		// Reset the extra args if already set, not to break `helm fetch` by adding the args intended for `lint`
		helm.SetExtraArgs()

		if len(args) > 0 {
			helm.SetExtraArgs(args...)
		}

		opts := &state.LintOpts{
			Set:         c.Set(),
			SkipCleanup: c.SkipCleanup(),
		}
		lintErrs := st.LintReleases(helm, c.Values(), args, c.Concurrency(), opts)
		if len(lintErrs) == 1 {
			if err, ok := lintErrs[0].(helmexec.ExitError); ok {
				if err.Code > 0 {
					deferredLintErrs = append(deferredLintErrs, err)

					return nil
				}
			}
		}

		return lintErrs
	})

	return ok, deferredLintErrs, errs
}

func (a *App) status(r *Run, c StatusesConfigProvider) (bool, []error) {
	st := r.state
	helm := r.helm

	allReleases := st.Releases

	selectedReleases, selectedAndNeededReleases, err := a.getSelectedReleases(r, false)
	if err != nil {
		return false, []error{err}
	}
	if len(selectedReleases) == 0 {
		return false, nil
	}

	// Do build deps and prepare only on selected releases so that we won't waste time
	// on running various helm commands on unnecessary releases
	st.Releases = selectedAndNeededReleases

	var toStatus []state.ReleaseSpec
	for _, r := range selectedReleases {
		if r.Desired() {
			toStatus = append(toStatus, r)
		}
	}

	var errs []error

	// Traverse DAG of all the releases so that we don't suffer from false-positive missing dependencies
	st.Releases = allReleases

	args := GetArgs(c.Args(), st)

	// Reset the extra args if already set, not to break `helm fetch` by adding the args intended for `lint`
	helm.SetExtraArgs()

	if len(args) > 0 {
		helm.SetExtraArgs(args...)
	}

	if len(toStatus) > 0 {
		_, templateErrs := withDAG(st, helm, a.Logger, state.PlanOptions{SelectedReleases: toStatus, Reverse: false, SkipNeeds: true}, a.WrapWithoutSelector(func(subst *state.HelmState, helm helmexec.Interface) []error {
			return subst.ReleaseStatuses(helm, c.Concurrency())
		}))

		if len(templateErrs) > 0 {
			errs = append(errs, templateErrs...)
		}
	}
	return true, errs
}

func (a *App) sync(r *Run, c SyncConfigProvider) (bool, []error) {
	st := r.state
	helm := r.helm

	selectedReleases, selectedAndNeededReleases, err := a.getSelectedReleases(r, c.IncludeTransitiveNeeds())
	if err != nil {
		return false, []error{err}
	}
	if len(selectedReleases) == 0 {
		return false, nil
	}

	// This is required when you're trying to deduplicate releases by the selector.
	// Without this, `PlanReleases` conflates duplicates and return both in `batches`,
	// even if we provided `SelectedReleases: selectedReleases`.
	// See https://github.com/roboll/helmfile/issues/1818 for more context.
	st.Releases = selectedAndNeededReleases

	batches, err := st.PlanReleases(state.PlanOptions{Reverse: false, SelectedReleases: selectedReleases, IncludeNeeds: c.IncludeNeeds(), IncludeTransitiveNeeds: c.IncludeTransitiveNeeds(), SkipNeeds: c.SkipNeeds()})
	if err != nil {
		return false, []error{err}
	}

	var toSyncWithNeeds []state.ReleaseSpec

	for _, rs := range batches {
		for _, r := range rs {
			toSyncWithNeeds = append(toSyncWithNeeds, r.ReleaseSpec)
		}
	}

	// Do build deps and prepare only on selected releases so that we won't waste time
	// on running various helm commands on unnecessary releases
	st.Releases = toSyncWithNeeds

	toDelete, err := st.DetectReleasesToBeDeletedForSync(helm, toSyncWithNeeds)
	if err != nil {
		return false, []error{err}
	}

	releasesToDelete := map[string]state.ReleaseSpec{}
	for _, r := range toDelete {
		release := r
		id := state.ReleaseToID(&release)
		releasesToDelete[id] = release
	}

	var toUpdate []state.ReleaseSpec
	for _, r := range toSyncWithNeeds {
		release := r
		if _, deleted := releasesToDelete[state.ReleaseToID(&release)]; !deleted {
			if r.Desired() {
				toUpdate = append(toUpdate, release)
			}
			// TODO Emit error when the user opted to fail when the needed release is disabled,
			// instead of silently ignoring it.
			// See https://github.com/roboll/helmfile/issues/1018
		}
	}

	releasesToUpdate := map[string]state.ReleaseSpec{}
	for _, r := range toUpdate {
		release := r
		id := state.ReleaseToID(&release)
		releasesToUpdate[id] = release
	}

	releasesWithNoChange := map[string]state.ReleaseSpec{}
	for _, r := range toSyncWithNeeds {
		release := r
		id := state.ReleaseToID(&release)
		_, uninstalled := releasesToDelete[id]
		_, updated := releasesToUpdate[id]
		if !uninstalled && !updated {
			releasesWithNoChange[id] = release
		}
	}

	for id := range releasesWithNoChange {
		r := releasesWithNoChange[id]
		if _, err := st.TriggerCleanupEvent(&r, "sync"); err != nil {
			a.Logger.Warnf("warn: %v\n", err)
		}
	}

	names := []string{}
	for _, r := range releasesToUpdate {
		names = append(names, fmt.Sprintf("  %s (%s) UPDATED", r.Name, r.Chart))
	}
	for _, r := range releasesToDelete {
		names = append(names, fmt.Sprintf("  %s (%s) DELETED", r.Name, r.Chart))
	}
	// Make the output deterministic for testing purpose
	sort.Strings(names)

	infoMsg := fmt.Sprintf(`Affected releases are:
%s
`, strings.Join(names, "\n"))

	confMsg := fmt.Sprintf(`%s
Do you really want to sync?
  Helmfile will sync all your releases, as shown above.

`, infoMsg)

	interactive := c.Interactive()
	if !interactive {
		a.Logger.Debug(infoMsg)
	}

	var errs []error

	r.helm.SetExtraArgs(GetArgs(c.Args(), r.state)...)

	// Traverse DAG of all the releases so that we don't suffer from false-positive missing dependencies
	st.Releases = selectedAndNeededReleases

	affectedReleases := state.AffectedReleases{}

	if !interactive || interactive && r.askForConfirmation(confMsg) {
		if len(releasesToDelete) > 0 {
			_, deletionErrs := withDAG(st, helm, a.Logger, state.PlanOptions{Reverse: true, SelectedReleases: toDelete, SkipNeeds: true}, a.WrapWithoutSelector(func(subst *state.HelmState, helm helmexec.Interface) []error {
				var rs []state.ReleaseSpec

				for _, r := range subst.Releases {
					release := r
					if r2, ok := releasesToDelete[state.ReleaseToID(&release)]; ok {
						rs = append(rs, r2)
					}
				}

				subst.Releases = rs

				return subst.DeleteReleasesForSync(&affectedReleases, helm, c.Concurrency(), c.Cascade())
			}))

			if len(deletionErrs) > 0 {
				errs = append(errs, deletionErrs...)
			}
		}

		if len(releasesToUpdate) > 0 {
			_, syncErrs := withDAG(st, helm, a.Logger, state.PlanOptions{SelectedReleases: toUpdate, SkipNeeds: true, IncludeTransitiveNeeds: c.IncludeTransitiveNeeds()}, a.WrapWithoutSelector(func(subst *state.HelmState, helm helmexec.Interface) []error {
				var rs []state.ReleaseSpec

				for _, r := range subst.Releases {
					release := r
					if _, ok := releasesToDelete[state.ReleaseToID(&release)]; !ok {
						rs = append(rs, release)
					}
				}

				subst.Releases = rs

				opts := &state.SyncOpts{
					Set:                  c.Set(),
					SkipCRDs:             c.SkipCRDs(),
					Wait:                 c.Wait(),
					WaitRetries:          c.WaitRetries(),
					WaitForJobs:          c.WaitForJobs(),
					ReuseValues:          c.ReuseValues(),
					ResetValues:          c.ResetValues(),
					PostRenderer:         c.PostRenderer(),
					PostRendererArgs:     c.PostRendererArgs(),
					SyncArgs:             c.SyncArgs(),
					HideNotes:            c.HideNotes(),
					TakeOwnership:        c.TakeOwnership(),
					SkipSchemaValidation: c.SkipSchemaValidation(),
				}
				return subst.SyncReleases(&affectedReleases, helm, c.Values(), c.Concurrency(), opts)
			}))

			if len(syncErrs) > 0 {
				errs = append(errs, syncErrs...)
			}
		}
	}
	affectedReleases.DisplayAffectedReleases(c.Logger())
	return true, errs
}

func (a *App) template(r *Run, c TemplateConfigProvider) (bool, []error) {
	return a.withNeeds(r, c, false, func(st *state.HelmState) []error {
		helm := r.helm

		args := GetArgs(c.Args(), st)

		// Reset the extra args if already set, not to break `helm fetch` by adding the args intended for `lint`
		helm.SetExtraArgs()

		if len(args) > 0 {
			helm.SetExtraArgs(args...)
		}

		opts := &state.TemplateOpts{
			Set:                  c.Set(),
			IncludeCRDs:          c.IncludeCRDs(),
			NoHooks:              c.NoHooks(),
			OutputDirTemplate:    c.OutputDirTemplate(),
			SkipCleanup:          c.SkipCleanup(),
			SkipTests:            c.SkipTests(),
			PostRenderer:         c.PostRenderer(),
			PostRendererArgs:     c.PostRendererArgs(),
			KubeVersion:          c.KubeVersion(),
			ShowOnly:             c.ShowOnly(),
			SkipSchemaValidation: c.SkipSchemaValidation(),
		}
		return st.TemplateReleases(helm, c.OutputDir(), c.Values(), args, c.Concurrency(), c.Validate(), opts)
	})
}

func (a *App) withNeeds(r *Run, c DAGConfig, includeDisabled bool, f func(*state.HelmState) []error) (bool, []error) {
	st := r.state

	selectedReleases, deduplicated, err := a.getSelectedReleases(r, false)
	if err != nil {
		return false, []error{err}
	}
	if len(selectedReleases) == 0 {
		return false, nil
	}

	// This is required when you're trying to deduplicate releases by the selector.
	// Without this, `PlanReleases` conflates duplicates and return both in `batches`,
	// even if we provided `SelectedReleases: selectedReleases`.
	// See https://github.com/roboll/helmfile/issues/1818 for more context.
	st.Releases = deduplicated

	includeNeeds := c.IncludeNeeds()
	if c.IncludeTransitiveNeeds() {
		includeNeeds = true
	}

	batches, err := st.PlanReleases(state.PlanOptions{Reverse: false, SelectedReleases: selectedReleases, IncludeNeeds: includeNeeds, IncludeTransitiveNeeds: c.IncludeTransitiveNeeds(), SkipNeeds: c.SkipNeeds()})
	if err != nil {
		return false, []error{err}
	}

	var selectedReleasesWithNeeds []state.ReleaseSpec

	for _, rs := range batches {
		for _, r := range rs {
			selectedReleasesWithNeeds = append(selectedReleasesWithNeeds, r.ReleaseSpec)
		}
	}

	var toRender []state.ReleaseSpec

	releasesToUninstall := map[string]state.ReleaseSpec{}
	for _, r := range selectedReleasesWithNeeds {
		release := r
		id := state.ReleaseToID(&release)
		if !release.Desired() {
			releasesToUninstall[id] = release
		} else {
			toRender = append(toRender, release)
		}
	}

	var rels []state.ReleaseSpec

	if len(toRender) > 0 {
		// toRender already contains the direct and transitive needs depending on the DAG options.
		// That's why we don't pass in `IncludeNeeds: c.IncludeNeeds(), IncludeTransitiveNeeds: c.IncludeTransitiveNeeds()` here.
		// Otherwise, in case include-needs=true, it will include the needs of needs, which results in unexpectedly introducing transitive needs,
		// even if include-transitive-needs=true is unspecified.
		if _, errs := withDAG(st, r.helm, a.Logger, state.PlanOptions{SelectedReleases: toRender, Reverse: false, SkipNeeds: c.SkipNeeds(), IncludeNeeds: includeNeeds}, a.WrapWithoutSelector(func(subst *state.HelmState, helm helmexec.Interface) []error {
			rels = append(rels, subst.Releases...)
			return nil
		})); len(errs) > 0 {
			return false, errs
		}
	}

	if includeDisabled {
		for _, d := range releasesToUninstall {
			rels = append(rels, d)
		}
	}

	// Traverse DAG of all the releases so that we don't suffer from false-positive missing dependencies
	// and we don't fail on dependenciese on disabled releases.
	// In diff, we need to diff on disabled releases to show to-be-uninstalled releases.
	// In lint and template, we'd need to run respective helm commands only on enabled releases,
	// without failing on disabled releases.
	st.Releases = rels

	errs := f(st)

	return true, errs
}

func (a *App) test(r *Run, c TestConfigProvider) []error {
	cleanup := c.Cleanup()
	timeout := c.Timeout()
	concurrency := c.Concurrency()

	st := r.state

	toTest, _, err := a.getSelectedReleases(r, false)
	if err != nil {
		return []error{err}
	}

	if len(toTest) == 0 {
		return nil
	}

	// Do test only on selected releases, because that's what the user intended
	// with conditions and selectors
	st.Releases = toTest

	r.helm.SetExtraArgs(GetArgs(c.Args(), r.state)...)

	return st.TestReleases(r.helm, cleanup, timeout, concurrency, state.Logs(c.Logs()))
}

func (a *App) writeValues(r *Run, c WriteValuesConfigProvider) (bool, []error) {
	st := r.state
	helm := r.helm

	toRender, _, err := a.getSelectedReleases(r, false)
	if err != nil {
		return false, []error{err}
	}
	if len(toRender) == 0 {
		return false, nil
	}

	// Do build deps and prepare only on selected releases so that we won't waste time
	// on running various helm commands on unnecessary releases
	st.Releases = toRender

	releasesToWrite := map[string]state.ReleaseSpec{}
	for _, r := range toRender {
		release := r
		id := state.ReleaseToID(&release)
		if release.Desired() {
			releasesToWrite[id] = release
		}
	}

	var errs []error

	// Note: We don't calculate the DAG of releases here unlike other helmfile operations,
	// because there's no need to do so for just writing values.
	// See the first bullet in https://github.com/roboll/helmfile/issues/1460#issuecomment-691863465
	if len(releasesToWrite) > 0 {
		var rs []state.ReleaseSpec

		for _, r := range releasesToWrite {
			rs = append(rs, r)
		}

		st.Releases = rs

		opts := &state.WriteValuesOpts{
			Set:                c.Set(),
			OutputFileTemplate: c.OutputFileTemplate(),
			SkipCleanup:        c.SkipCleanup(),
		}
		errs = st.WriteReleasesValues(helm, c.Values(), opts)
	}

	return true, errs
}

// Error is a wrapper around an error that adds context to the error.
type Error struct {
	msg string

	Errors []error

	code *int
}

func (e *Error) Error() string {
	var cause string
	if e.Errors == nil {
		return e.msg
	}
	if len(e.Errors) == 1 {
		if e.Errors[0] == nil {
			panic(fmt.Sprintf("[bug] assertion error: unexpected state: e.Errors: %v", e.Errors))
		}
		cause = e.Errors[0].Error()
	} else {
		msgs := []string{}
		for i, err := range e.Errors {
			if err == nil {
				continue
			}
			msgs = append(msgs, fmt.Sprintf("err %d: %v", i, err.Error()))
		}
		cause = fmt.Sprintf("%d errors:\n%s", len(e.Errors), strings.Join(msgs, "\n"))
	}
	msg := ""
	if e.msg != "" {
		msg = fmt.Sprintf("%s: %s", e.msg, cause)
	} else {
		msg = cause
	}
	return msg
}

func (e *Error) Code() int {
	if e.code != nil {
		return *e.code
	}

	allDiff := false
	anyNonZero := false
	for _, err := range e.Errors {
		switch ee := err.(type) {
		case *state.ReleaseError:
			if anyNonZero {
				allDiff = allDiff && ee.Code == 2
			} else {
				allDiff = ee.Code == 2
			}
		case *Error:
			if anyNonZero {
				allDiff = allDiff && ee.Code() == 2
			} else {
				allDiff = ee.Code() == 2
			}
		}
		anyNonZero = true
	}

	if anyNonZero {
		if allDiff {
			return 2
		}
		return 1
	}
	panic(fmt.Sprintf("[bug] assertion error: unexpected state: unable to handle errors: %v", e.Errors))
}

func appError(msg string, err error) *Error {
	return &Error{msg: msg, Errors: []error{err}}
}

func (c context) clean(errs []error) error {
	if errs == nil {
		errs = []error{}
	}

	if !c.retainValues {
		cleanErrs := c.st.Clean()
		if cleanErrs != nil {
			errs = append(errs, cleanErrs...)
		}
	}

	return c.wrapErrs(errs...)
}

type context struct {
	app *App
	st  *state.HelmState

	retainValues bool
}

func (c context) wrapErrs(errs ...error) error {
	if len(errs) > 0 {
		for _, err := range errs {
			switch e := err.(type) {
			case *state.ReleaseError:
				c.app.Logger.Debugf("err: release \"%s\" in \"%s\" failed: %v", e.Name, c.st.FilePath, e)
			default:
				c.app.Logger.Debugf("err: %v", e)
			}
		}
		return &Error{Errors: errs}
	}
	return nil
}

func (a *App) ShowCacheDir(c CacheConfigProvider) error {
	fmt.Printf("Cache directory: %s\n", remote.CacheDir())

	if !a.fs.DirectoryExistsAt(remote.CacheDir()) {
		return nil
	}
	dirs, err := a.fs.ReadDir(remote.CacheDir())
	if err != nil {
		return err
	}
	for _, e := range dirs {
		fmt.Printf("- %s\n", e.Name())
	}

	return nil
}

func (a *App) CleanCacheDir(c CacheConfigProvider) error {
	if !a.fs.DirectoryExistsAt(remote.CacheDir()) {
		return nil
	}
	fmt.Printf("Cleaning up cache directory: %s\n", remote.CacheDir())
	dirs, err := os.ReadDir(remote.CacheDir())
	if err != nil {
		return err
	}
	for _, e := range dirs {
		fmt.Printf("- %s\n", e.Name())
		err := os.RemoveAll(filepath.Join(remote.CacheDir(), e.Name()))
		if err != nil {
			return err
		}
	}

	return nil
}

func GetArgs(args string, state *state.HelmState) []string {
	baseArgs := []string{}
	stateArgs := []string{}
	if len(args) > 0 {
		baseArgs = argparser.CollectArgs(args)
	}

	if len(state.HelmDefaults.Args) > 0 {
		stateArgs = argparser.CollectArgs(strings.Join(state.HelmDefaults.Args, " "))
	}
	state.HelmDefaults.Args = append(baseArgs, stateArgs...)

	return state.HelmDefaults.Args
}
