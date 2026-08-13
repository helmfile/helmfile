package app

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/helmfile/vals"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

// prefetchTrackingHelm wraps exectest.Helm to verify the real production
// wiring behind issue #2741 (app.go's PrefetchSharedRemoteCharts:true on
// Diff/Sync/Apply -> state.PrepareCharts -> state.withChartOperationLock):
// it counts Fetch calls and records the chart argument each DiffRelease call
// actually received.
type prefetchTrackingHelm struct {
	*exectest.Helm

	fetchCount atomic.Int32

	mu          sync.Mutex
	diffedChart map[string]string
}

func (m *prefetchTrackingHelm) Fetch(chart string, flags ...string) error {
	untarDir := ""
	for i, f := range flags {
		if f == "--untardir" && i+1 < len(flags) {
			untarDir = flags[i+1]
			break
		}
	}
	if untarDir != "" {
		chartDir := filepath.Join(untarDir, chart)
		_ = os.MkdirAll(chartDir, 0755)
		_ = os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: test\nversion: 1.0.0\n"), 0644)
	}
	m.fetchCount.Add(1)
	return nil
}

func (m *prefetchTrackingHelm) DiffRelease(context helmexec.HelmContext, name, chart, namespace string, suppressDiff bool, flags ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.diffedChart == nil {
		m.diffedChart = map[string]string{}
	}
	m.diffedChart[name] = chart
	return nil
}

// TestDiffPrefetchesSharedRemoteChart is the end-to-end regression test for
// issue #2741, exercising the real production wiring rather than a hand
// simulation of it: app.Diff -> run.WithPreparedCharts(...,
// PrefetchSharedRemoteCharts: true) -> state.PrepareCharts ->
// withChartOperationLock -> DiffRelease. Two releases share a chart+version
// from a declared repository; without the fix they'd each hand
// "myrepo/mychart" to helm-diff untouched (safe, but serialized by
// withChartOperationLock). With the fix, the chart is fetched once and both
// releases receive the same materialized local chart path, so
// withChartOperationLock's ChartPath != "" guard lets them run concurrently.
//
// This test uses a real on-disk helmfile.yaml and the real filesystem
// (ffs.DefaultFileSystem()), not the in-memory testhelper.TestFs used by
// sibling tests in this package: the chart-download cache's fast path
// (forcedDownloadChart, see state.go) corroborates a cache hit against disk
// via st.fs.DirectoryExistsAt, and the in-memory TestFs has no way to see
// files a mocked `helm fetch` writes to real disk, which would make every
// call look like a cache miss regardless of this fix.
func TestDiffPrefetchesSharedRemoteChart(t *testing.T) {
	tempDir := t.TempDir()
	helmfilePath := filepath.Join(tempDir, "helmfile.yaml")
	helmfileContent := []byte(`
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
`)
	require.NoError(t, os.WriteFile(helmfilePath, helmfileContent, 0644))

	logger := zap.NewExample().Sugar()

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	require.NoError(t, err)

	helm := &prefetchTrackingHelm{Helm: &exectest.Helm{
		ChartsMutex:   &sync.Mutex{},
		DiffMutex:     &sync.Mutex{},
		ReleasesMutex: &sync.Mutex{},
	}}

	app := &App{
		OverrideHelmBinary:              DefaultHelmBinary,
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		FileOrDir:                       helmfilePath,
		Logger:                          logger,
		fs:                              ffs.DefaultFileSystem(),
		Set:                             map[string]any{},
		helms: map[helmKey]helmexec.Interface{
			createHelmKey(DefaultHelmBinary, "default"): helm,
		},
		valsRuntime: valsRuntime,
	}

	diffErr := app.Diff(diffConfig{
		concurrency: 5,
		logger:      logger,
	})
	require.NoError(t, diffErr)

	require.Equal(t, int32(1), helm.fetchCount.Load(),
		"myrepo/mychart shared by 2 releases must be fetched exactly once")

	require.Len(t, helm.diffedChart, 2)
	chartA := helm.diffedChart["release-a"]
	chartB := helm.diffedChart["release-b"]
	require.NotEmpty(t, chartA)
	require.NotEqual(t, "myrepo/mychart", chartA, "release-a should receive the materialized local chart path, not the bare chart reference")
	require.Equal(t, chartA, chartB, "both releases sharing the chart should receive the identical materialized path")
}
