package state

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

const sharedChartHelmfile = `
repositories:
  - name: myrepo
    url: https://example.com/charts

releases:
  - name: release-a
    chart: myrepo/mychart
    version: 1.0.0
  - name: release-b
    chart: myrepo/mychart
    version: 1.0.0
  - name: release-c
    chart: myrepo/mychart
    version: 1.0.0
  - name: release-d
    chart: myrepo/mychart
    version: 1.0.0
  - name: release-e
    chart: myrepo/mychart
    version: 1.0.0
`

const mixedVerifyFlagsHelmfile = `
repositories:
  - name: myrepo
    url: https://example.com/charts

releases:
  - name: release-a
    chart: myrepo/mychart
    version: 1.0.0
    verify: true
  - name: release-b
    chart: myrepo/mychart
    version: 1.0.0
    verify: false
`

const uniformVerifyFlagsHelmfile = `
repositories:
  - name: myrepo
    url: https://example.com/charts

releases:
  - name: release-a
    chart: myrepo/mychart
    version: 1.0.0
    verify: true
  - name: release-b
    chart: myrepo/mychart
    version: 1.0.0
    verify: true
`

const localStyleChartNoRepoHelmfile = `
releases:
  - name: frontend-v2
    chart: charts/frontend
  - name: frontend-v3
    chart: charts/frontend
`

const uniqueChartsHelmfile = `
repositories:
  - name: myrepo
    url: https://example.com/charts

releases:
  - name: release-a
    chart: myrepo/chart-a
    version: 1.0.0
  - name: release-b
    chart: myrepo/chart-b
    version: 1.0.0
  - name: release-c
    chart: myrepo/chart-c
    version: 1.0.0
`

// TestPrepareChartsPrefetchesSharedRemoteChart verifies that when
// PrefetchSharedRemoteCharts is set, releases sharing a remote chart+version
// trigger exactly one `helm fetch`, and all of them are handed a materialized
// local chart path. This is the fix for issue #2741.
func TestPrepareChartsPrefetchesSharedRemoteChart(t *testing.T) {
	resetChartCacheForTest()

	logger := zap.NewExample().Sugar()
	st, err := createFromYaml([]byte(sharedChartHelmfile), "example/path/to/helmfile.yaml", DefaultEnv, logger)
	require.NoError(t, err)

	tempDir := t.TempDir()

	mockHelm := &mockFetchHelm{Helm: &exectest.Helm{Helm3: true, ChartsMutex: &sync.Mutex{}}}

	opts := ChartPrepareOptions{
		SkipResolve:                true,
		PrefetchSharedRemoteCharts: true,
		Concurrency:                5,
		OutputDirTemplate:          "{{ .OutputDir }}/{{ .Release.Name }}",
	}

	releaseToChart, _, errs := st.PrepareCharts(mockHelm, tempDir, 5, "sync", opts)
	require.Empty(t, errs, "PrepareCharts should not return errors")

	assert.Equal(t, int32(1), mockHelm.fetchCount.Load(),
		"helm.Fetch must be called exactly once for 5 releases sharing the same chart+version")

	require.Len(t, releaseToChart, 5)

	var paths []string
	for _, name := range []string{"release-a", "release-b", "release-c", "release-d", "release-e"} {
		path, ok := releaseToChart[PrepareChartKey{Name: name}]
		require.True(t, ok, "release %s should have a prepared chart", name)
		assert.NotEqual(t, "myrepo/mychart", path, "release %s should have a materialized local chart path", name)
		paths = append(paths, path)
	}
	for _, p := range paths[1:] {
		assert.Equal(t, paths[0], p, "all releases sharing the chart should be prepared to the same local path")
	}
}

// TestPrepareChartsSkipsPrefetchForUniqueCharts verifies that
// PrefetchSharedRemoteCharts leaves charts used by only one release
// untouched: no forced download, no local ChartPath materialized. Preserves
// current behavior for the common single-release-per-chart case.
func TestPrepareChartsSkipsPrefetchForUniqueCharts(t *testing.T) {
	resetChartCacheForTest()

	logger := zap.NewExample().Sugar()
	st, err := createFromYaml([]byte(uniqueChartsHelmfile), "example/path/to/helmfile.yaml", DefaultEnv, logger)
	require.NoError(t, err)

	tempDir := t.TempDir()

	mockHelm := &mockFetchHelm{Helm: &exectest.Helm{Helm3: true, ChartsMutex: &sync.Mutex{}}}

	opts := ChartPrepareOptions{
		SkipResolve:                true,
		PrefetchSharedRemoteCharts: true,
		Concurrency:                3,
		OutputDirTemplate:          "{{ .OutputDir }}/{{ .Release.Name }}",
	}

	releaseToChart, _, errs := st.PrepareCharts(mockHelm, tempDir, 3, "sync", opts)
	require.Empty(t, errs, "PrepareCharts should not return errors")

	assert.Equal(t, int32(0), mockHelm.fetchCount.Load(),
		"helm.Fetch must not be called for charts used by only one release")

	for name, chart := range map[string]string{
		"release-a": "myrepo/chart-a",
		"release-b": "myrepo/chart-b",
		"release-c": "myrepo/chart-c",
	} {
		path, ok := releaseToChart[PrepareChartKey{Name: name}]
		require.True(t, ok)
		assert.Equal(t, chart, path, "unique chart should be handed to helm unmodified, not materialized locally")
	}
}

// TestPrepareChartsSkipsPrefetchWhenFetchFlagsDiffer verifies that releases
// sharing a chart+version are NOT prefetched if they resolve to different
// chart acquisition flags (verify/keyring/plain-http/insecure-skip-tls-verify).
// The download cache is keyed by chart+version alone, so deduplicating a
// fetch across releases with different --verify settings would let whichever
// release's worker wins the race silently decide verification for the other.
func TestPrepareChartsSkipsPrefetchWhenFetchFlagsDiffer(t *testing.T) {
	resetChartCacheForTest()

	logger := zap.NewExample().Sugar()
	st, err := createFromYaml([]byte(mixedVerifyFlagsHelmfile), "example/path/to/helmfile.yaml", DefaultEnv, logger)
	require.NoError(t, err)

	tempDir := t.TempDir()

	mockHelm := &mockFetchHelm{Helm: &exectest.Helm{Helm3: true, ChartsMutex: &sync.Mutex{}}}

	opts := ChartPrepareOptions{
		SkipResolve:                true,
		PrefetchSharedRemoteCharts: true,
		Concurrency:                2,
		OutputDirTemplate:          "{{ .OutputDir }}/{{ .Release.Name }}",
	}

	releaseToChart, _, errs := st.PrepareCharts(mockHelm, tempDir, 2, "sync", opts)
	require.Empty(t, errs)

	assert.Equal(t, int32(0), mockHelm.fetchCount.Load(),
		"releases with differing verify/keyring/TLS flags must not be deduplicated into a single fetch")

	for _, name := range []string{"release-a", "release-b"} {
		path, ok := releaseToChart[PrepareChartKey{Name: name}]
		require.True(t, ok)
		assert.Equal(t, "myrepo/mychart", path, "chart should be handed to helm unmodified when flags differ across releases")
	}
}

// TestPrepareChartsSkipsPrefetchWhenVerifyEnabled verifies that releases
// sharing a chart+version are NOT prefetched when verify is enabled, even
// when every release agrees on identical flags. forcedDownloadChart untars
// the chart into a local directory, but flagsForUpgrade unconditionally
// re-adds --verify for the later `helm upgrade` regardless of ChartPath —
// and Helm's VerifyChart only accepts a packaged .tgz/provenance pair, not
// an unpacked directory, so upgrading a prefetched+verified chart would fail
// with a real helm binary. Confirms both that prefetch is skipped and that
// the resulting flagsForUpgrade output for the affected release still
// carries --verify unchanged, i.e. behaves exactly as it did before this
// feature existed.
func TestPrepareChartsSkipsPrefetchWhenVerifyEnabled(t *testing.T) {
	resetChartCacheForTest()

	logger := zap.NewExample().Sugar()
	st, err := createFromYaml([]byte(uniformVerifyFlagsHelmfile), "example/path/to/helmfile.yaml", DefaultEnv, logger)
	require.NoError(t, err)

	tempDir := t.TempDir()

	mockHelm := &mockFetchHelm{Helm: &exectest.Helm{Helm3: true, ChartsMutex: &sync.Mutex{}}}

	opts := ChartPrepareOptions{
		SkipResolve:                true,
		PrefetchSharedRemoteCharts: true,
		Concurrency:                2,
		OutputDirTemplate:          "{{ .OutputDir }}/{{ .Release.Name }}",
	}

	releaseToChart, _, errs := st.PrepareCharts(mockHelm, tempDir, 2, "sync", opts)
	require.Empty(t, errs)

	assert.Equal(t, int32(0), mockHelm.fetchCount.Load(),
		"verify-enabled releases must not be prefetched, even with identical flags across the group")

	var release *ReleaseSpec
	for i := range st.Releases {
		if st.Releases[i].Name == "release-a" {
			release = &st.Releases[i]
		}
	}
	require.NotNil(t, release)

	path, ok := releaseToChart[PrepareChartKey{Name: "release-a"}]
	require.True(t, ok)
	assert.Equal(t, "myrepo/mychart", path, "chart should be handed to helm unmodified, not materialized locally")
	assert.Empty(t, release.ChartPath, "ChartPath must stay unset so withChartOperationLock keeps serializing this chart")

	// flagsForUpgrade's own --verify emission is unconditional and untouched by
	// this feature (see state.go:4052-4055) — it's driven purely by
	// release.Verify/repo.Verify/HelmDefaults.Verify, not by ChartPath. Because
	// prefetch was skipped above, that pre-existing logic is exactly what runs
	// for this release, exactly as it did before this feature existed.
	flags := st.appendVerifyFlags(nil, release)
	assert.Contains(t, flags, "--verify", "verify must still reach the upgrade command exactly as before this feature existed")
}

// TestPrepareChartsSkipsPrefetchForUnknownRepoChart verifies that a
// "dir/chart"-shaped chart string with no matching `repositories:` entry
// (the conventional way to reference a local chart, e.g. "charts/frontend")
// is never force-downloaded even when shared by multiple releases. Locks in
// the fix for the regression this caused in pkg/app's diff/sync fixtures,
// where such releases must be handed unmodified to helm/diff, not routed
// through forcedDownloadChart.
func TestPrepareChartsSkipsPrefetchForUnknownRepoChart(t *testing.T) {
	resetChartCacheForTest()

	logger := zap.NewExample().Sugar()
	st, err := createFromYaml([]byte(localStyleChartNoRepoHelmfile), "example/path/to/helmfile.yaml", DefaultEnv, logger)
	require.NoError(t, err)

	tempDir := t.TempDir()

	mockHelm := &mockFetchHelm{Helm: &exectest.Helm{Helm3: true, ChartsMutex: &sync.Mutex{}}}

	opts := ChartPrepareOptions{
		SkipResolve:                true,
		PrefetchSharedRemoteCharts: true,
		Concurrency:                2,
		OutputDirTemplate:          "{{ .OutputDir }}/{{ .Release.Name }}",
	}

	releaseToChart, _, errs := st.PrepareCharts(mockHelm, tempDir, 2, "diff", opts)
	require.Empty(t, errs)

	assert.Equal(t, int32(0), mockHelm.fetchCount.Load(),
		"a chart string with no matching repository must not be treated as a remote chart")

	for _, name := range []string{"frontend-v2", "frontend-v3"} {
		path, ok := releaseToChart[PrepareChartKey{Name: name}]
		require.True(t, ok)
		assert.Equal(t, "charts/frontend", path)
	}
}

// concurrencyTrackingHelm wraps exectest.Helm to record the maximum number of
// concurrent SyncRelease/DiffRelease calls observed.
type concurrencyTrackingHelm struct {
	*exectest.Helm

	syncConcurrent atomic.Int32
	syncMax        atomic.Int32
	diffConcurrent atomic.Int32
	diffMax        atomic.Int32
}

func bumpMax(cur *atomic.Int32, max *atomic.Int32) {
	c := cur.Add(1)
	for {
		old := max.Load()
		if c <= old || max.CompareAndSwap(old, c) {
			break
		}
	}
	time.Sleep(20 * time.Millisecond)
	cur.Add(-1)
}

func (m *concurrencyTrackingHelm) SyncRelease(context helmexec.HelmContext, name, chart, namespace string, flags ...string) error {
	bumpMax(&m.syncConcurrent, &m.syncMax)
	return nil
}

func (m *concurrencyTrackingHelm) DiffRelease(context helmexec.HelmContext, name, chart, namespace string, suppressDiff bool, flags ...string) error {
	bumpMax(&m.diffConcurrent, &m.diffMax)
	return nil
}

// TestPrefetchedSharedChartAllowsConcurrentSyncRelease is the end-to-end
// regression test for issue #2741: it runs PrepareCharts with
// PrefetchSharedRemoteCharts on 5 releases sharing a remote chart, applies the
// resulting chart paths to each release (mirroring what
// app.Run.WithPreparedCharts does), then drives them through
// withChartOperationLock + SyncRelease exactly like the production sync path
// (see state.go's SyncRelease call sites). Without the prefetch, this would
// serialize to max concurrency 1 (see TestWithChartOperationLockSerializesSameChart);
// with it, all 5 should run concurrently.
func TestPrefetchedSharedChartAllowsConcurrentSyncRelease(t *testing.T) {
	resetChartCacheForTest()

	logger := zap.NewExample().Sugar()
	st, err := createFromYaml([]byte(sharedChartHelmfile), "example/path/to/helmfile.yaml", DefaultEnv, logger)
	require.NoError(t, err)

	tempDir := t.TempDir()

	fetchHelm := &mockFetchHelm{Helm: &exectest.Helm{Helm3: true, ChartsMutex: &sync.Mutex{}}}

	opts := ChartPrepareOptions{
		SkipResolve:                true,
		PrefetchSharedRemoteCharts: true,
		Concurrency:                5,
		OutputDirTemplate:          "{{ .OutputDir }}/{{ .Release.Name }}",
	}

	releaseToChart, _, errs := st.PrepareCharts(fetchHelm, tempDir, 5, "sync", opts)
	require.Empty(t, errs)
	require.Equal(t, int32(1), fetchHelm.fetchCount.Load())

	// Mirror app.Run.WithPreparedCharts: only set ChartPath when the prepared
	// chart differs from the original chart reference.
	for i := range st.Releases {
		rel := &st.Releases[i]
		if chart, ok := releaseToChart[PrepareChartKey{Name: rel.Name}]; ok && chart != rel.Chart {
			rel.ChartPath = chart
		}
	}

	trackingHelm := &concurrencyTrackingHelm{Helm: &exectest.Helm{}}

	var wg sync.WaitGroup
	for i := range st.Releases {
		rel := &st.Releases[i]
		require.NotEmpty(t, rel.ChartPath, "release %s should have a materialized ChartPath after prefetch", rel.Name)
		wg.Add(1)
		go func(release *ReleaseSpec) {
			defer wg.Done()
			_ = st.withChartOperationLock(release, release.Chart, func() error {
				return trackingHelm.SyncRelease(helmexec.HelmContext{}, release.Name, release.ChartPath, release.Namespace)
			})
		}(rel)
	}
	wg.Wait()

	assert.Equal(t, int32(5), trackingHelm.syncMax.Load(),
		"prefetched shared-chart releases must run SyncRelease concurrently, not serialized")
}

// TestPrefetchedSharedChartAllowsConcurrentDiffRelease is the DiffRelease
// analog of TestPrefetchedSharedChartAllowsConcurrentSyncRelease, mirroring
// the withChartOperationLock(release, chartPath, DiffRelease) call site in
// prepareDiffReleases.
func TestPrefetchedSharedChartAllowsConcurrentDiffRelease(t *testing.T) {
	resetChartCacheForTest()

	logger := zap.NewExample().Sugar()
	st, err := createFromYaml([]byte(sharedChartHelmfile), "example/path/to/helmfile.yaml", DefaultEnv, logger)
	require.NoError(t, err)

	tempDir := t.TempDir()

	fetchHelm := &mockFetchHelm{Helm: &exectest.Helm{Helm3: true, ChartsMutex: &sync.Mutex{}}}

	opts := ChartPrepareOptions{
		SkipResolve:                true,
		PrefetchSharedRemoteCharts: true,
		Concurrency:                5,
		OutputDirTemplate:          "{{ .OutputDir }}/{{ .Release.Name }}",
	}

	releaseToChart, _, errs := st.PrepareCharts(fetchHelm, tempDir, 5, "diff", opts)
	require.Empty(t, errs)
	require.Equal(t, int32(1), fetchHelm.fetchCount.Load())

	for i := range st.Releases {
		rel := &st.Releases[i]
		if chart, ok := releaseToChart[PrepareChartKey{Name: rel.Name}]; ok && chart != rel.Chart {
			rel.ChartPath = chart
		}
	}

	trackingHelm := &concurrencyTrackingHelm{Helm: &exectest.Helm{}}

	var wg sync.WaitGroup
	for i := range st.Releases {
		rel := &st.Releases[i]
		require.NotEmpty(t, rel.ChartPath)
		wg.Add(1)
		go func(release *ReleaseSpec) {
			defer wg.Done()
			_ = st.withChartOperationLock(release, release.ChartPath, func() error {
				return trackingHelm.DiffRelease(helmexec.HelmContext{}, release.Name, release.ChartPath, release.Namespace, false)
			})
		}(rel)
	}
	wg.Wait()

	assert.Equal(t, int32(5), trackingHelm.diffMax.Load(),
		"prefetched shared-chart releases must run DiffRelease concurrently, not serialized")
}
