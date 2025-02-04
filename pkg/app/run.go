package app

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/fatih/color"

	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/state"
)

type Run struct {
	state *state.HelmState
	helm  helmexec.Interface
	ctx   Context

	ReleaseToChart map[state.PrepareChartKey]string

	Ask func(string) bool
}

func NewRun(st *state.HelmState, helm helmexec.Interface, ctx Context) (*Run, error) {
	if helm == nil {
		return nil, fmt.Errorf("Assertion failed: helmexec.Interface must not be nil")
	}

	if !helm.IsHelm3() {
		return nil, fmt.Errorf("helmfile has deprecated helm2 since v0.150.0")
	}

	return &Run{state: st, helm: helm, ctx: ctx}, nil
}

func (r *Run) askForConfirmation(msg string) bool {
	if r.Ask != nil {
		return r.Ask(msg)
	}
	return AskForConfirmation(msg)
}

func (r *Run) prepareChartsIfNeeded(helmfileCommand string, dir string, concurrency int, opts state.ChartPrepareOptions) (map[state.PrepareChartKey]string, error) {
	// Skip chart preparation for certain commands
	skipCommands := []string{"write-values", "list"}
	if slices.Contains(skipCommands, strings.ToLower(helmfileCommand)) {
		return nil, nil
	}

	releaseToChart, errs := r.state.PrepareCharts(r.helm, dir, concurrency, helmfileCommand, opts)
	if len(errs) > 0 {
		return nil, fmt.Errorf("%v", errs)
	}

	return releaseToChart, nil
}

func (r *Run) withPreparedCharts(helmfileCommand string, opts state.ChartPrepareOptions, f func()) error {
	if r.ReleaseToChart != nil {
		panic("Run.PrepareCharts can be called only once")
	}

	if !opts.SkipRepos {
		ctx := r.ctx
		if err := ctx.SyncReposOnce(r.state, r.helm); err != nil {
			return err
		}
	}

	// Create tmp directory and bail immediately if it fails
	var dir string
	if len(opts.OutputDir) == 0 {
		tempDir, err := os.MkdirTemp("", "helmfile*")
		if err != nil {
			return err
		}
		defer func() {
			_ = os.RemoveAll(tempDir)
		}()
		dir = tempDir
	} else {
		dir = opts.OutputDir
		fmt.Printf("Charts will be downloaded to: %s\n", dir)
	}

	if _, err := r.state.TriggerGlobalPrepareEvent(helmfileCommand); err != nil {
		return err
	}

	releaseToChart, err := r.prepareChartsIfNeeded(helmfileCommand, dir, opts.Concurrency, opts)
	if err != nil {
		return err
	}

	for i := range r.state.Releases {
		rel := &r.state.Releases[i]
		key := state.PrepareChartKey{
			Name:        rel.Name,
			Namespace:   rel.Namespace,
			KubeContext: rel.KubeContext,
		}
		if chart := releaseToChart[key]; chart != rel.Chart {
			// The chart has been downloaded and modified by Helmfile (and chartify under the hood).
			// We let the later step use the modified version of the chart, located under the `chart` variable,
			// instead of the original chart path.
			// This way, the later step can use the modified chart without knowing
			// if it has been modified or not.
			rel.ChartPath = chart
		}
	}

	r.ReleaseToChart = releaseToChart

	f()

	_, err = r.state.TriggerGlobalCleanupEvent(helmfileCommand)
	return err
}

func (r *Run) Deps(c DepsConfigProvider) []error {
	if !c.SkipRepos() {
		if err := r.ctx.SyncReposOnce(r.state, r.helm); err != nil {
			return []error{err}
		}
	}

	r.helm.SetExtraArgs(GetArgs(c.Args(), r.state)...)

	return r.state.UpdateDeps(r.helm, c.IncludeTransitiveNeeds())
}

func (r *Run) Repos(c ReposConfigProvider) error {
	r.helm.SetExtraArgs(GetArgs(c.Args(), r.state)...)

	return r.ctx.SyncReposOnce(r.state, r.helm)
}

func (r *Run) diff(triggerCleanupEvent bool, detailedExitCode bool, c DiffConfigProvider, diffOpts *state.DiffOpts) (*string, map[string]state.ReleaseSpec, map[string]state.ReleaseSpec, []error) {
	st := r.state
	helm := r.helm

	var changedReleases []state.ReleaseSpec
	var deletingReleases []state.ReleaseSpec
	var planningErrs []error

	// TODO Better way to detect diff on only filtered releases
	{
		changedReleases, planningErrs = st.DiffReleases(helm, c.Values(), c.Concurrency(), detailedExitCode, c.StripTrailingCR(), c.IncludeTests(), c.Suppress(), c.SuppressSecrets(), c.ShowSecrets(), c.NoHooks(), c.SuppressDiff(), triggerCleanupEvent, diffOpts)

		var err error
		deletingReleases, err = st.DetectReleasesToBeDeletedForSync(helm, st.Releases)
		if err != nil {
			planningErrs = append(planningErrs, err)
		}
	}

	fatalErrs := []error{}

	for _, e := range planningErrs {
		switch err := e.(type) {
		case *state.ReleaseError:
			if err.Code != 2 {
				fatalErrs = append(fatalErrs, e)
			}
		default:
			fatalErrs = append(fatalErrs, e)
		}
	}

	if len(fatalErrs) > 0 {
		return nil, nil, nil, fatalErrs
	}

	releasesToBeDeleted := map[string]state.ReleaseSpec{}
	for _, r := range deletingReleases {
		release := r
		id := state.ReleaseToID(&release)
		releasesToBeDeleted[id] = release
	}

	releasesToBeUpdated := map[string]state.ReleaseSpec{}
	for _, r := range changedReleases {
		release := r
		id := state.ReleaseToID(&release)

		// If `helm-diff` detected changes but it is not being `helm delete`ed, we should run `helm upgrade`
		if _, ok := releasesToBeDeleted[id]; !ok {
			releasesToBeUpdated[id] = release
		}
	}

	// sync only when there are changes
	if len(releasesToBeUpdated) == 0 && len(releasesToBeDeleted) == 0 {
		var msg *string
		if c.DetailedExitcode() {
			// TODO better way to get the logger
			m := "No affected releases"
			msg = &m
		}
		return msg, nil, nil, nil
	}

	names := []string{}
	for _, r := range releasesToBeUpdated {
		names = append(names, fmt.Sprintf("  %s (%s) UPDATED", r.Name, r.Chart))
	}
	for _, r := range releasesToBeDeleted {
		releaseToBeDeleted := fmt.Sprintf("  %s (%s) DELETED", r.Name, r.Chart)
		if c.Color() {
			releaseToBeDeleted = color.RedString(releaseToBeDeleted)
		}
		names = append(names, releaseToBeDeleted)
	}
	// Make the output deterministic for testing purpose
	sort.Strings(names)

	infoMsg := fmt.Sprintf(`Affected releases are:
%s
`, strings.Join(names, "\n"))

	return &infoMsg, releasesToBeUpdated, releasesToBeDeleted, nil
}
