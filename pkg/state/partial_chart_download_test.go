package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	"github.com/helmfile/helmfile/pkg/filesystem"
)

// partialFetchHelm simulates `helm pull --untar` dying part-way through the
// extraction: Chart.yaml (the first entry in a chart archive) lands on disk,
// nothing after it does, and the fetch reports an error.
type partialFetchHelm struct {
	*exectest.Helm
}

func (h *partialFetchHelm) Fetch(chart string, flags ...string) error {
	var untarDir string
	for i, f := range flags {
		if f == "--untardir" && i+1 < len(flags) {
			untarDir = flags[i+1]
			break
		}
	}
	if untarDir != "" {
		chartDir := filepath.Join(untarDir, "mychart")
		if err := os.MkdirAll(chartDir, 0755); err != nil {
			return err
		}
		partial := "apiVersion: v2\nname: mychart\nversion: 1.0.0\n"
		if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(partial), 0644); err != nil {
			return err
		}
	}
	return errors.New("fetch failed")
}

// A failed fetch must not leave a partially extracted chart in the cache.
//
// Chart.yaml is the first entry in a chart archive, so an interrupted extraction
// typically leaves a complete Chart.yaml and nothing else. findChartDirectory
// accepts any directory containing a Chart.yaml, so such leftovers are adopted as
// a valid cache entry by the next run -- which then renders zero manifests and
// exits 0. Under --skip-refresh, or anywhere below the shared cache dir where
// isSharedCachePath suppresses refresh, that state is permanent.
func TestForcedDownloadChart_RemovesPartialDownloadOnFetchError(t *testing.T) {
	cacheRoot := t.TempDir()

	st := &HelmState{
		fs:       filesystem.DefaultFileSystem(),
		logger:   zap.NewNop().Sugar(),
		Releases: []ReleaseSpec{{Name: "app", Chart: "myrepo/mychart"}},
	}

	release := &st.Releases[0]
	helm := &partialFetchHelm{Helm: &exectest.Helm{}}

	_, err := st.forcedDownloadChart("myrepo/mychart", cacheRoot, release, helm, ChartPrepareOptions{
		SkipRefresh: true,
	})
	require.Error(t, err, "a failed fetch must surface as an error")

	entries, readErr := os.ReadDir(cacheRoot)
	require.NoError(t, readErr)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		leftover := filepath.Join(cacheRoot, e.Name())
		_, findErr := findChartDirectory(leftover)
		require.Error(t, findErr,
			"partial download at %s was left behind and would be reused as a valid chart", leftover)
	}
}
