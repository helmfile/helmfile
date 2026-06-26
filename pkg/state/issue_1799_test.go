package state

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/filesystem"
)

// TestChartifyTempDirCleanup verifies that chartify output directories tracked
// via addChartifyTempDir are removed by CleanupChartifyTempDirs, including the
// parent temp directory when it becomes empty. Regression test for issue #1799.
func TestChartifyTempDirCleanup(t *testing.T) {
	st := &HelmState{
		logger: zap.NewNop().Sugar(),
		fs:     filesystem.DefaultFileSystem(),
	}

	// Simulate a chartify output directory: /tmp/chartify<rand>/<id>
	parent := t.TempDir()
	dir := filepath.Join(parent, "release-name-12345")
	require.NoError(t, os.MkdirAll(dir, 0755))
	// Add some content so RemoveAll has something to remove
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("apiVersion: v2\n"), 0644))

	st.addChartifyTempDir(dir)

	// Before cleanup, the directory exists
	_, err := os.Stat(dir)
	assert.NoError(t, err)

	st.CleanupChartifyTempDirs()

	// After cleanup, the chartify output dir is gone
	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err), "chartify output dir should be removed")

	// The parent temp dir should also be removed since it is now empty
	_, err = os.Stat(parent)
	assert.True(t, os.IsNotExist(err), "empty parent temp dir should be removed")
}

// TestChartifyTempDirCleanupMultipleReleases verifies that cleanup handles
// multiple chartify output directories correctly, including shared parents.
func TestChartifyTempDirCleanupMultipleReleases(t *testing.T) {
	st := &HelmState{
		logger: zap.NewNop().Sugar(),
		fs:     filesystem.DefaultFileSystem(),
	}

	// Two chartify output dirs under the SAME parent (simulating concurrent
	// releases that share /tmp/chartify<rand>/)
	parent := t.TempDir()
	dir1 := filepath.Join(parent, "release-a-111")
	dir2 := filepath.Join(parent, "release-b-222")
	require.NoError(t, os.MkdirAll(dir1, 0755))
	require.NoError(t, os.MkdirAll(dir2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "Chart.yaml"), []byte("apiVersion: v2\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "Chart.yaml"), []byte("apiVersion: v2\n"), 0644))

	st.addChartifyTempDir(dir1)
	st.addChartifyTempDir(dir2)

	st.CleanupChartifyTempDirs()

	for _, d := range []string{dir1, dir2} {
		_, err := os.Stat(d)
		assert.True(t, os.IsNotExist(err), "chartify output dir %s should be removed", d)
	}
	// Parent should also be cleaned since both children are gone
	_, err := os.Stat(parent)
	assert.True(t, os.IsNotExist(err), "parent should be removed when all children removed")
}

// TestChartifyTempDirCleanupNoop verifies CleanupChartifyTempDirs is safe to call
// when no directories were tracked (e.g. no chartify runs occurred).
func TestChartifyTempDirCleanupNoop(t *testing.T) {
	st := &HelmState{
		logger: zap.NewNop().Sugar(),
		fs:     filesystem.DefaultFileSystem(),
	}

	// Should not panic
	assert.NotPanics(t, func() {
		st.CleanupChartifyTempDirs()
	})
}

// TestChartifyTempDirCleanupIdempotent verifies that calling cleanup twice is safe.
func TestChartifyTempDirCleanupIdempotent(t *testing.T) {
	st := &HelmState{
		logger: zap.NewNop().Sugar(),
		fs:     filesystem.DefaultFileSystem(),
	}

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("apiVersion: v2\n"), 0644))

	st.addChartifyTempDir(dir)
	st.CleanupChartifyTempDirs()
	// Second call should be a no-op, not an error
	assert.NotPanics(t, func() {
		st.CleanupChartifyTempDirs()
	})
}

// TestChartifyTempDirConcurrentTracking verifies that concurrent calls to
// addChartifyTempDir (as happens with parallel chart-preparation workers) do
// not lose any tracked directories. The tracker must be pre-initialized before
// concurrent access — see PrepareCharts. Regression test for issue #1799.
func TestChartifyTempDirConcurrentTracking(t *testing.T) {
	st := &HelmState{
		logger: zap.NewNop().Sugar(),
		fs:     filesystem.DefaultFileSystem(),
	}

	// Pre-initialize the tracker as PrepareCharts does before launching workers.
	st.chartifyTempDirs = &chartifyTempDirTracker{}

	const n = 50
	dirs := make([]string, n)
	for i := range dirs {
		dirs[i] = t.TempDir()
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(dir string) {
			defer wg.Done()
			st.addChartifyTempDir(dir)
		}(dirs[i])
	}
	wg.Wait()

	// All n directories must be tracked — none lost to a race.
	st.chartifyTempDirs.mu.Lock()
	got := len(st.chartifyTempDirs.dirs)
	st.chartifyTempDirs.mu.Unlock()
	assert.Equal(t, n, got, "all concurrently tracked dirs must be present")

	// Cleanup should remove all of them.
	st.CleanupChartifyTempDirs()
	for i, d := range dirs {
		_, err := os.Stat(d)
		assert.True(t, os.IsNotExist(err), "dir %d (%s) should have been removed", i, d)
	}
}
