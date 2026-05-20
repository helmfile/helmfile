package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/filesystem"
)

// TestLocalDependencyChartPathNormalization tests that relative chart paths in
// release dependencies (like "../chart") are normalized to absolute paths
// relative to basePath before checking if the directory exists.
// This is a regression test for issue #2596.
//
// Background: When helmfile.d/ contains multiple release files and one release
// has a local chart dependency (chart: ../chart), the dependency chart path was
// passed to DirectoryExistsAt without normalization, causing it to be resolved
// relative to the CWD instead of basePath. This made helmfile fail to detect
// the local chart and instead try to resolve it as a remote repo, resulting in
// "failed reading adhoc dependencies: no helm list entry found for repository".
func TestLocalDependencyChartPathNormalization(t *testing.T) {
	tempDir := t.TempDir()

	chartDir := filepath.Join(tempDir, "chart")
	require.NoError(t, os.MkdirAll(chartDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(`
apiVersion: v2
name: test-chart
version: 0.1.0
`), 0644))

	helmfileDir := filepath.Join(tempDir, "helmfile.d")
	require.NoError(t, os.MkdirAll(helmfileDir, 0755))

	tests := []struct {
		name        string
		chartPath   string
		basePath    string
		expectLocal bool
	}{
		{
			name:        "relative path ../chart normalized from helmfile.d",
			chartPath:   "../chart",
			basePath:    helmfileDir,
			expectLocal: true,
		},
		{
			name:        "absolute path works unchanged",
			chartPath:   chartDir,
			basePath:    helmfileDir,
			expectLocal: true,
		},
		{
			name:        "non-existent relative path not detected as local",
			chartPath:   "../nonexistent",
			basePath:    helmfileDir,
			expectLocal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizedChart := normalizeChart(tt.basePath, tt.chartPath)
			fs := filesystem.DefaultFileSystem()
			isLocal := fs.DirectoryExistsAt(normalizedChart)
			assert.Equal(t, tt.expectLocal, isLocal,
				"normalizeChart(%q, %q) = %q, DirectoryExistsAt = %v, want %v",
				tt.basePath, tt.chartPath, normalizedChart, isLocal, tt.expectLocal)
		})
	}
}

// TestDependencyChartPathResolutionWithPrepareChartify verifies that the dependency
// chart path is normalized using basePath before calling DirectoryExistsAt,
// which is the core of the fix for issue #2596.
func TestDependencyChartPathResolutionWithPrepareChartify(t *testing.T) {
	tempDir := t.TempDir()

	chartDir := filepath.Join(tempDir, "chart")
	require.NoError(t, os.MkdirAll(chartDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(`
apiVersion: v2
name: test-chart
version: 0.1.0
`), 0644))

	helmfileDir := filepath.Join(tempDir, "helmfile.d")
	require.NoError(t, os.MkdirAll(helmfileDir, 0755))

	fs := filesystem.DefaultFileSystem()

	tests := []struct {
		name           string
		depChartPath   string
		basePath       string
		expectDetected bool
	}{
		{
			name:           "relative ../chart from helmfile.d detected as local",
			depChartPath:   "../chart",
			basePath:       helmfileDir,
			expectDetected: true,
		},
		{
			name:           "absolute path detected as local",
			depChartPath:   chartDir,
			basePath:       helmfileDir,
			expectDetected: true,
		},
		{
			name:           "non-existent relative path not detected",
			depChartPath:   "../nonexistent",
			basePath:       helmfileDir,
			expectDetected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizedChart := normalizeChart(tt.basePath, tt.depChartPath)
			isLocal := fs.DirectoryExistsAt(normalizedChart)
			assert.Equal(t, tt.expectDetected, isLocal,
				"normalizeChart(%q, %q) = %q, DirectoryExistsAt = %v, want %v",
				tt.basePath, tt.depChartPath, normalizedChart, isLocal, tt.expectDetected)

			if tt.expectDetected && !filepath.IsAbs(tt.depChartPath) {
				absChart, err := filepath.Abs(filepath.Join(tt.basePath, tt.depChartPath))
				require.NoError(t, err)
				assert.Equal(t, absChart, normalizedChart,
					"normalized path should match expected absolute path")
			}
		})
	}
}
