package app

import (
	"fmt"
	"sync"
	"testing"

	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/state"
)

func fakeConverge(failures map[string]bool, executed *[]string) func(*state.HelmState, helmexec.Interface) (bool, []error) {
	return func(st *state.HelmState, _ helmexec.Interface) (bool, []error) {
		var errs []error
		for i := range st.Releases {
			release := &st.Releases[i]
			*executed = append(*executed, release.Name)
			if failures[release.Name] {
				err := fmt.Errorf("failed processing release %s: boom", release.Name)
				errs = append(errs, state.NewReleaseError(release, err, state.ReleaseErrorCodeFailure))
			}
		}

		return len(st.Releases) > 0, errs
	}
}

func TestWithBatches_FailFastByDefault(t *testing.T) {
	templated := &state.HelmState{}
	var executed []string

	batches := [][]state.Release{
		{
			state.Release{Name: "database", Needs: nil, ContinueOnError: nil},
			state.Release{Name: "cache", Needs: nil, ContinueOnError: nil},
		},
		{
			state.Release{Name: "worker", Needs: nil, ContinueOnError: nil},
		},
	}

	any, errs := withBatches("processing", templated, batches, &exectest.Helm{}, zap.NewNop().Sugar(), fakeConverge(map[string]bool{"database": true}, &executed))

	require.False(t, any)
	require.Len(t, errs, 1)
	require.Equal(t, []string{"database", "cache"}, executed)
	require.Equal(t, "failed processing release database: boom", errs[0].Error())
}

func TestWithBatches_ContinueOnErrorKeepsIndependentBranches(t *testing.T) {
	templated := &state.HelmState{}
	var executed []string

	batches := [][]state.Release{
		{
			state.Release{Name: "database", Needs: nil, ContinueOnError: new(true)},
			state.Release{Name: "cache", Needs: nil, ContinueOnError: nil},
		},
		{
			state.Release{Name: "worker", Needs: nil, ContinueOnError: nil},
		},
	}

	any, errs := withBatches("processing", templated, batches, &exectest.Helm{}, zap.NewNop().Sugar(), fakeConverge(map[string]bool{"database": true}, &executed))

	require.True(t, any)
	require.Len(t, errs, 1)
	require.Equal(t, []string{"database", "cache", "worker"}, executed)
	require.Equal(t, "failed processing release database: boom", errs[0].Error())
}

func TestWithBatches_BlocksDependentReleasesAndAggregatesErrors(t *testing.T) {
	templated := &state.HelmState{}
	var executed []string

	batches := [][]state.Release{
		{
			state.Release{Name: "database", Needs: nil, ContinueOnError: new(true)},
		},
		{
			state.Release{Name: "backend", Needs: []string{"database"}, ContinueOnError: new(true)},
			state.Release{Name: "worker", Needs: nil, ContinueOnError: nil},
		},
		{
			state.Release{Name: "frontend", Needs: []string{"backend"}, ContinueOnError: new(true)},
		},
	}

	any, errs := withBatches("processing", templated, batches, &exectest.Helm{}, zap.NewNop().Sugar(), fakeConverge(map[string]bool{"database": true}, &executed))

	require.True(t, any)
	require.Len(t, errs, 3)
	require.Equal(t, []string{"database", "worker"}, executed)
	require.Equal(t, "failed processing release database: boom", errs[0].Error())
	require.Contains(t, errs[1].Error(), "release \"backend\" was skipped because dependency \"database\" failed")
	require.Contains(t, errs[2].Error(), "release \"frontend\" was skipped because dependency \"backend\" failed")
}

func TestWithBatches_AggregatesMultipleReleaseErrors(t *testing.T) {
	templated := &state.HelmState{}
	var executed []string

	batches := [][]state.Release{
		{
			state.Release{Name: "app-a", Needs: nil, ContinueOnError: new(true)},
			state.Release{Name: "app-b", Needs: nil, ContinueOnError: new(true)},
		},
		{
			state.Release{Name: "worker", Needs: nil, ContinueOnError: nil},
		},
	}

	any, errs := withBatches("processing", templated, batches, &exectest.Helm{}, zap.NewNop().Sugar(), fakeConverge(map[string]bool{"app-a": true, "app-b": true}, &executed))

	require.True(t, any)
	require.Len(t, errs, 2)
	require.Equal(t, []string{"app-a", "app-b", "worker"}, executed)
	for _, releaseName := range []string{"app-a", "app-b"} {
		require.Contains(t, fmt.Sprint(errs), releaseName)
	}
}

func TestWithBatches_TreatsNonFailureReleaseErrorCodeAsFatal(t *testing.T) {
	templated := &state.HelmState{}
	var executed []string

	// e.g. the helm-diff "changes detected" exit code 2, which is not a
	// release failure and must never enable continuation.
	converge := func(st *state.HelmState, _ helmexec.Interface) (bool, []error) {
		var errs []error
		for i := range st.Releases {
			release := &st.Releases[i]
			executed = append(executed, release.Name)
			errs = append(errs, state.NewReleaseError(release, fmt.Errorf("release %s: changed", release.Name), 2))
		}
		return false, errs
	}

	batches := [][]state.Release{
		{
			state.Release{Name: "database", Needs: nil, ContinueOnError: new(true)},
		},
		{
			state.Release{Name: "worker", Needs: nil, ContinueOnError: nil},
		},
	}

	any, errs := withBatches("processing", templated, batches, &exectest.Helm{}, zap.NewNop().Sugar(), converge)

	require.False(t, any)
	require.Len(t, errs, 1)
	require.Equal(t, []string{"database"}, executed)
}

// TestSync_ContinueOnError exercises the whole sync pipeline with the exectest
// fake helm: the release named "error-*" fails to upgrade, and continueOnError
// on that release must (1) let the independent release still be deployed,
// (2) skip the dependent release with an explicit error, and (3) keep the
// overall command failing with a non-zero exit code.
func TestSync_ContinueOnError(t *testing.T) {
	newApp := func(t *testing.T, helm *exectest.Helm, files map[string]string, logger *zap.SugaredLogger) *App {
		t.Helper()

		valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
		require.NoError(t, err)

		return appWithFs(&App{
			OverrideHelmBinary:              DefaultHelmBinary,
			fs:                              ffs.DefaultFileSystem(),
			OverrideKubeContext:             "default",
			DisableKubeVersionAutoDetection: true,
			Env:                             "default",
			Logger:                          logger,
			helms: map[helmKey]helmexec.Interface{
				createHelmKey("helm", "default"): helm,
			},
			valsRuntime: valsRuntime,
		}, files)
	}

	t.Run("independent release continues and dependent release is skipped", func(t *testing.T) {
		var helm = &exectest.Helm{
			FailOnUnexpectedList: true,
			DiffMutex:            &sync.Mutex{},
			ChartsMutex:          &sync.Mutex{},
			ReleasesMutex:        &sync.Mutex{},
		}

		files := map[string]string{
			"/path/to/helmfile.yaml": `
releases:
- name: error-database
  chart: incubator/raw
  namespace: default
  continueOnError: true

- name: independent-app
  chart: incubator/raw
  namespace: default

- name: dependent-backend
  chart: incubator/raw
  namespace: default
  needs:
  - default/error-database
`,
		}

		logger := zap.NewNop().Sugar()

		app := newApp(t, helm, files, logger)
		syncErr := app.Sync(applyConfig{
			concurrency: 1,
			logger:      logger,
		})

		require.Error(t, syncErr, "sync must exit non-zero even when errors are tolerated")

		appErr, ok := syncErr.(*Error)
		require.True(t, ok, "expected *app.Error, got %T", syncErr)
		assert.Equal(t, 1, appErr.Code())

		// independent-app must have been deployed despite error-database failing
		synced := make([]string, 0, len(helm.Releases))
		for _, r := range helm.Releases {
			synced = append(synced, r.Name)
		}
		assert.Contains(t, synced, "independent-app")
		assert.NotContains(t, synced, "dependent-backend", "release depending on a failed release must be skipped")
		assert.NotContains(t, synced, "error-database")

		assert.Contains(t, syncErr.Error(), `release "dependent-backend" was skipped because dependency "error-database" failed`)
	})

	t.Run("fail-fast remains the default", func(t *testing.T) {
		var helm = &exectest.Helm{
			FailOnUnexpectedList: true,
			DiffMutex:            &sync.Mutex{},
			ChartsMutex:          &sync.Mutex{},
			ReleasesMutex:        &sync.Mutex{},
		}

		files := map[string]string{
			"/path/to/helmfile.yaml": `
releases:
- name: error-database
  chart: incubator/raw
  namespace: default

- name: base
  chart: incubator/raw
  namespace: default

- name: later-app
  chart: incubator/raw
  namespace: default
  needs:
  - default/base
`,
		}

		logger := zap.NewNop().Sugar()

		app := newApp(t, helm, files, logger)
		syncErr := app.Sync(applyConfig{
			concurrency: 1,
			logger:      logger,
		})

		require.Error(t, syncErr)

		synced := make([]string, 0, len(helm.Releases))
		for _, r := range helm.Releases {
			synced = append(synced, r.Name)
		}
		// base shares the first batch with error-database and is therefore
		// still attempted, but the second batch must never run once the
		// failure is reported.
		assert.Contains(t, synced, "base")
		assert.NotContains(t, synced, "later-app", "without continueOnError the default fail-fast behavior must stop the run")
	})
}
