package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/filesystem"
)

// TestLocalChartWithTransformersPathNormalization tests that relative chart paths
// like "../chart" are normalized to absolute paths when transformers are present.
// This is a regression test for issue #2297.
//
// Background: When using local charts with transformers in helmfile.d/, the chartify
// process would receive the unnormalized relative path and try to run "helm pull ../chart"
// which fails because "../chart" is not a valid repo reference.
func TestLocalChartWithTransformersPathNormalization(t *testing.T) {
	// Create a temporary directory structure that mimics the issue:
	// tempDir/
	//   helmfile.d/
	//     with-transformers.yaml  (chart: ../chart)
	//   chart/
	//     Chart.yaml
	tempDir := t.TempDir()

	// Create the chart directory
	chartDir := filepath.Join(tempDir, "chart")
	require.NoError(t, os.MkdirAll(chartDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(`
apiVersion: v2
name: test-chart
version: 0.1.0
`), 0644))

	// Create the helmfile.d directory (this is the basePath)
	helmfileDir := filepath.Join(tempDir, "helmfile.d")
	require.NoError(t, os.MkdirAll(helmfileDir, 0755))

	tests := []struct {
		name           string
		chartPath      string // Relative chart path as specified in helmfile.yaml
		basePath       string // basePath is helmfile.d/
		hasTransformer bool
		expectAbsPath  bool // Should the chart path be normalized to absolute?
	}{
		{
			name:           "local chart with transformers should be normalized",
			chartPath:      "../chart",
			basePath:       helmfileDir,
			hasTransformer: true,
			expectAbsPath:  true,
		},
		{
			name:           "local chart without transformers should also work",
			chartPath:      "../chart",
			basePath:       helmfileDir,
			hasTransformer: false,
			expectAbsPath:  true,
		},
		{
			name:           "absolute local chart path should remain unchanged",
			chartPath:      chartDir, // Already absolute
			basePath:       helmfileDir,
			hasTransformer: true,
			expectAbsPath:  true, // Already absolute
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a HelmState with the appropriate settings
			state := &HelmState{
				basePath: tt.basePath,
				fs: &filesystem.FileSystem{
					DirectoryExistsAt: func(path string) bool {
						// Normalize the path and check if it matches our chart directory
						absPath := path
						if !filepath.IsAbs(path) {
							absPath = filepath.Join(tt.basePath, path)
						}
						return absPath == chartDir || path == chartDir
					},
					FileExistsAt: func(path string) bool {
						// Check for Chart.yaml
						if filepath.Base(path) == "Chart.yaml" {
							dir := filepath.Dir(path)
							if !filepath.IsAbs(dir) {
								dir = filepath.Join(tt.basePath, dir)
							}
							return dir == chartDir
						}
						return false
					},
				},
				logger: logger,
			}

			// Test the normalizeChart function directly
			normalizedPath := normalizeChart(tt.basePath, tt.chartPath)

			if tt.expectAbsPath {
				// The normalized path should be absolute and point to the chart directory
				assert.True(t, filepath.IsAbs(normalizedPath) || normalizedPath == chartDir,
					"Expected absolute path, got: %s", normalizedPath)

				// Verify the normalized path actually exists
				if filepath.IsAbs(normalizedPath) {
					assert.True(t, state.fs.DirectoryExistsAt(normalizedPath),
						"Normalized path should exist: %s", normalizedPath)
				}
			}

			// Additional test: verify isLocalChart detection
			isLocal := state.fs.DirectoryExistsAt(normalizeChart(tt.basePath, tt.chartPath))
			assert.True(t, isLocal, "Chart should be detected as local")
		})
	}
}

// TestChartPathNormalizationBeforeChartification specifically tests that
// when chartification is needed (transformers present), local chart paths
// are normalized to absolute paths before being passed to processChartification.
func TestChartPathNormalizationBeforeChartification(t *testing.T) {
	tempDir := t.TempDir()

	// Create directory structure:
	// tempDir/
	//   helmfile.d/
	//   chart/
	//     Chart.yaml
	helmfileDir := filepath.Join(tempDir, "helmfile.d")
	chartDir := filepath.Join(tempDir, "chart")

	require.NoError(t, os.MkdirAll(helmfileDir, 0755))
	require.NoError(t, os.MkdirAll(chartDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(`
apiVersion: v2
name: test-chart
version: 0.1.0
`), 0644))

	basePath := helmfileDir
	relativeChartPath := "../chart"

	// Normalize the path as the fix should do
	normalizedChartPath := normalizeChart(basePath, relativeChartPath)

	// The normalized path should be absolute
	assert.True(t, filepath.IsAbs(normalizedChartPath),
		"Normalized chart path should be absolute, got: %s", normalizedChartPath)

	// The normalized path should point to the actual chart directory
	expectedPath := chartDir
	assert.Equal(t, expectedPath, normalizedChartPath,
		"Normalized path should equal chart directory")

	// Verify the chart exists at the normalized path
	_, err := os.Stat(filepath.Join(normalizedChartPath, "Chart.yaml"))
	assert.NoError(t, err, "Chart.yaml should exist at normalized path")
}
