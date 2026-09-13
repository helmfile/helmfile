package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	"github.com/helmfile/helmfile/pkg/filesystem"
)

// mockFetchHelm wraps exectest.Helm to count Fetch calls and create
// a fake chart directory so findChartDirectory succeeds.
type mockFetchHelm struct {
	*exectest.Helm
	fetchCount atomic.Int32
}

func (m *mockFetchHelm) Fetch(chart string, flags ...string) error {
	// Extract the --untardir path from flags and create a fake Chart.yaml
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
		chartYaml := filepath.Join(chartDir, "Chart.yaml")
		_ = os.WriteFile(chartYaml, []byte("apiVersion: v2\nname: test\nversion: 1.0.0\n"), 0644)
	}
	m.fetchCount.Add(1)
	// Simulate download time to widen the race window
	time.Sleep(50 * time.Millisecond)
	return nil
}

// TestForcedDownloadChartSerializesSameChart verifies that concurrent calls
// to forcedDownloadChart with the same chart+version but different release
// names are serialized so helm.Fetch is called exactly once.
// This is the core fix for issue #768.
func TestForcedDownloadChartSerializesSameChart(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	tempDir := t.TempDir()

	// Use unique chart name to avoid polluting global cache across tests
	chartName := fmt.Sprintf("testrepo-768/unique-chart-%d", time.Now().UnixNano())

	st := &HelmState{
		logger:   logger.Sugar(),
		fs:       filesystem.DefaultFileSystem(),
		basePath: tempDir,
	}

	mockHelm := &mockFetchHelm{Helm: &exectest.Helm{}}

	numReleases := 5
	var wg sync.WaitGroup

	for i := 0; i < numReleases; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			release := &ReleaseSpec{
				Name:    fmt.Sprintf("release-%d", idx),
				Chart:   chartName,
				Version: "1.0.0",
			}
			opts := ChartPrepareOptions{
				SkipRefresh: true,
				SkipDeps:    true,
			}
			_, err := st.forcedDownloadChart(chartName, tempDir, release, mockHelm, opts)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// helm.Fetch should be called exactly once despite 5 concurrent releases
	assert.Equal(t, int32(1), mockHelm.fetchCount.Load(),
		"helm.Fetch must be called exactly once for concurrent releases with the same chart+version")
}

// TestForcedDownloadChartAllowsDifferentCharts verifies that downloads of
// different charts are NOT serialized against each other (only same-chart
// downloads are serialized).
func TestForcedDownloadChartAllowsDifferentCharts(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	tempDir := t.TempDir()
	ts := time.Now().UnixNano()

	st := &HelmState{
		logger:   logger.Sugar(),
		fs:       filesystem.DefaultFileSystem(),
		basePath: tempDir,
	}

	mockHelm := &mockFetchHelm{Helm: &exectest.Helm{}}

	// Two different charts
	charts := []string{
		fmt.Sprintf("testrepo-768/chart-a-%d", ts),
		fmt.Sprintf("testrepo-768/chart-b-%d", ts),
	}

	var wg sync.WaitGroup
	for _, chartName := range charts {
		wg.Add(1)
		go func(cn string) {
			defer wg.Done()
			release := &ReleaseSpec{
				Name:    "release-" + cn,
				Chart:   cn,
				Version: "1.0.0",
			}
			opts := ChartPrepareOptions{
				SkipRefresh: true,
				SkipDeps:    true,
			}
			_, e := st.forcedDownloadChart(cn, tempDir, release, mockHelm, opts)
			assert.NoError(t, e)
		}(chartName)
	}

	wg.Wait()

	// Both different charts should be fetched
	assert.Equal(t, int32(2), mockHelm.fetchCount.Load(),
		"different charts should not block each other")
}

// TestWithChartOperationLockSerializesSameChart verifies that
// withChartOperationLock serializes concurrent helm operations for
// releases using the same remote chart (issue #768).
func TestWithChartOperationLockSerializesSameChart(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	ts := time.Now().UnixNano()
	chartName := fmt.Sprintf("testrepo-768/op-chart-%d", ts)

	st := &HelmState{
		logger: logger.Sugar(),
	}

	var concurrentCount atomic.Int32
	var maxConcurrent atomic.Int32

	numReleases := 5
	var wg sync.WaitGroup

	for i := 0; i < numReleases; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			release := &ReleaseSpec{
				Name:    fmt.Sprintf("release-%d", idx),
				Chart:   chartName,
				Version: "1.0.0",
				// ChartPath empty => remote chart => lock acquired
			}
			_ = st.withChartOperationLock(release, chartName, func() error {
				cur := concurrentCount.Add(1)
				for {
					old := maxConcurrent.Load()
					if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				concurrentCount.Add(-1)
				return nil
			})
		}(i)
	}

	wg.Wait()

	// Same-chart operations must be serialized: max concurrency should be 1
	assert.Equal(t, int32(1), maxConcurrent.Load(),
		"concurrent helm operations for the same chart must be serialized")
}

// TestWithChartOperationLockNoLockForLocalChart verifies that
// withChartOperationLock does NOT serialize operations when ChartPath
// is set (local or pre-fetched charts).
func TestWithChartOperationLockNoLockForLocalChart(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	ts := time.Now().UnixNano()
	chartName := fmt.Sprintf("testrepo-768/local-chart-%d", ts)

	st := &HelmState{
		logger: logger.Sugar(),
	}

	var concurrentCount atomic.Int32
	var maxConcurrent atomic.Int32

	numReleases := 5
	var wg sync.WaitGroup

	for i := 0; i < numReleases; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			release := &ReleaseSpec{
				Name:      fmt.Sprintf("release-%d", idx),
				Chart:     chartName,
				Version:   "1.0.0",
				ChartPath: "/some/local/path", // ChartPath set => no lock
			}
			_ = st.withChartOperationLock(release, chartName, func() error {
				cur := concurrentCount.Add(1)
				for {
					old := maxConcurrent.Load()
					if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				concurrentCount.Add(-1)
				return nil
			})
		}(i)
	}

	wg.Wait()

	// Local charts should NOT be serialized: all should run concurrently
	assert.Greater(t, maxConcurrent.Load(), int32(1),
		"local/pre-fetched charts should not be serialized")
}
