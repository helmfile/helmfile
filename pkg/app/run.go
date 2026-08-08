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
	ctx   *Context

	ReleaseToChart map[state.PrepareChartKey]string

	Ask func(string) bool
}

func NewRun(st *state.HelmState, helm helmexec.Interface, ctx *Context) (*Run, error) {
	if helm == nil {
		return nil, fmt.Errorf("Assertion failed: helmexec.Interface must not be nil")
	}

	if !helm.IsHelm3() && !helm.IsHelm4() {
		return nil, fmt.Errorf("helmfile requires helm 3.x or 4.x")
	}

	return &Run{state: st, helm: helm, ctx: ctx}, nil
}

func (r *Run) askForConfirmation(msg string) bool {
	if r.Ask != nil {
		return r.Ask(msg)
	}
	return AskForConfirmation(msg)
}

// commandsSkipChartPrep lists commands that don't prepare or pull charts.
// These commands skip chart preparation, and when skipRepos is true they also
// skip the OCI-only registry login (since no chart pulls need authentication).
// When skipRepos is false, SyncReposOnce still runs normally for all repos.
var commandsSkipChartPrep = []string{"write-values", "list"}

func (r *Run) prepareChartsIfNeeded(helmfileCommand string, dir string, concurrency int, opts state.ChartPrepareOptions) (map[state.PrepareChartKey]string, error) {
	// Skip chart preparation for commands that don't need chart pulls
	if slices.Contains(commandsSkipChartPrep, strings.ToLower(helmfileCommand)) {
		return nil, nil
	}

	releaseToChart, errs := r.state.PrepareCharts(r.helm, dir, concurrency, helmfileCommand, opts)
	if len(errs) > 0 {
		if !opts.AllowFailedReleases {
			// abort on first error
			return nil, &MultiError{Errors: errs}
		} else {
			// return partial results with errors for the failed ones
			return releaseToChart, &MultiError{Errors: errs}
		}
	}

	return releaseToChart, nil
}

func (r *Run) WithPreparedCharts(helmfileCommand string, opts state.ChartPrepareOptions, f func() []error) error {
	if r.ReleaseToChart != nil {
		return fmt.Errorf("Run.WithPreparedCharts can be called only once")
	}

	// Check both CLI options and helmDefaults for skipping repos (issue #2296)
	// Both skipDeps and skipRefresh cause repo sync to be skipped because:
	// - skipRefresh explicitly means "don't update repos"
	// - skipDeps implies "I have all dependencies locally" which means repo data isn't needed
	// This matches the CLI behavior where --skip-deps and --skip-refresh both skip repo operations.
	// However, OCI registries need `helm registry login` before chart pulls when
	// credentials are configured (issue #1847), so when skipRepos is true we still
	// perform OCI-only login — but only for commands that actually pull charts.
	needsChartPrep := !slices.Contains(commandsSkipChartPrep, strings.ToLower(helmfileCommand))
	skipRepos := opts.SkipRepos || r.state.HelmDefaults.SkipDeps || r.state.HelmDefaults.SkipRefresh
	if !skipRepos {
		if err := r.ctx.SyncReposOnce(r.state, r.helm); err != nil {
			return err
		}
	} else if needsChartPrep {
		if err := r.ctx.SyncReposOnce(r.state, r.helm, state.WithOCIOnly()); err != nil {
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
		fmt.Fprintf(os.Stderr, "Charts will be downloaded to: %s\n", dir)
	}

	if _, err := r.state.TriggerGlobalPrepareEvent(helmfileCommand); err != nil {
		return err
	}

	// Ensure chartify temp directories are always cleaned up, even when chart
	// preparation or helm operations fail. The deferred call runs at function
	// exit — after f() and TriggerGlobalCleanupEvent — so chartified charts
	// remain available for the entire operation lifecycle. See issue #1799.
	defer r.state.CleanupChartifyTempDirs()

	releaseToChart, prepareErr := r.prepareChartsIfNeeded(helmfileCommand, dir, opts.Concurrency, opts)
	// IMPORTANT: on opts.AllowFailedReleases: do not abort only on error here, just forward it to the caller in order to allow for partial results
	// Only in case prepareCharts failed with a general error and returned to processable release abort anyways
	if prepareErr != nil && releaseToChart == nil {
		return prepareErr
	}
	if !opts.AllowFailedReleases {
		if prepareErr != nil {
			return prepareErr
		}
	}

	for i := range r.state.Releases {
		rel := &r.state.Releases[i]
		key := state.PrepareChartKey{
			Name:        rel.Name,
			Namespace:   rel.Namespace,
			KubeContext: rel.KubeContext,
		}
		if chart, ok := releaseToChart[key]; ok && chart != rel.Chart {
			// The chart has been downloaded and modified by Helmfile (and chartify under the hood).
			// We let the later step use the modified version of the chart, located under the `chart` variable,
			// instead of the original chart path.
			// This way, the later step can use the modified chart without knowing
			// if it has been modified or not.
			rel.ChartPath = chart
		}
	}

	r.ReleaseToChart = releaseToChart

	errs := f()
	var firstErr error
	for _, e := range errs {
		if e != nil {
			firstErr = e
			break
		}
	}

	_, cleanupErr := r.state.TriggerGlobalCleanupEvent(helmfileCommand, firstErr)
	if !opts.AllowFailedReleases {
		// return directly on first error
		return cleanupErr
	} else {
		// merge the two errors into a single error output
		var merged []error
		if prepareErr != nil {
			if me, ok := prepareErr.(*MultiError); ok {
				merged = append(merged, me.Errors...)
			} else {
				merged = append(merged, prepareErr)
			}
		}
		if cleanupErr != nil {
			merged = append(merged, fmt.Errorf("error during global cleanup event: %w", cleanupErr))
		}
		if len(merged) > 0 {
			return &MultiError{Errors: merged}
		}
		return nil
	}
}

func (r *Run) Deps(c DepsConfigProvider) []error {
	// Check both CLI options and helmDefaults for skipping repos (issue #2296)
	// Both skipDeps and skipRefresh cause repo sync to be skipped (see WithPreparedCharts for rationale).
	// OCI registries need login before chart pulls when credentials are
	// configured (issue #1847), so skipRepos=true still performs OCI-only login.
	skipRepos := c.SkipRepos() || r.state.HelmDefaults.SkipDeps || r.state.HelmDefaults.SkipRefresh
	var repoOpts []state.SyncOption
	if skipRepos {
		repoOpts = append(repoOpts, state.WithOCIOnly())
	}
	if err := r.ctx.SyncReposOnce(r.state, r.helm, repoOpts...); err != nil {
		return []error{err}
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

func (r *Run) State() *state.HelmState {
	return r.state
}

func (r *Run) Helm() helmexec.Interface {
	return r.helm
}
