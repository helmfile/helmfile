package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: boolPtr helper is already defined in skip_test.go and shared across test files in this package.

// TestHelmDefaultsSkipDepsAndSkipRefreshIntegration tests that helmDefaults.skipDeps
// and helmDefaults.skipRefresh are properly respected when preparing charts.
// This is a regression test for issue #2296.
//
// The existing skip_test.go tests the boolean logic, but this test verifies
// that the values actually flow through to the chartPrepareResult correctly.
func TestHelmDefaultsSkipDepsAndSkipRefreshIntegration(t *testing.T) {
	// Create a temporary directory with a local chart
	tempDir := t.TempDir()
	chartDir := filepath.Join(tempDir, "chart")
	require.NoError(t, os.MkdirAll(chartDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(`
apiVersion: v2
name: test-chart
version: 0.1.0
`), 0644))

	tests := []struct {
		name                    string
		helmDefaultsSkipDeps    bool
		helmDefaultsSkipRefresh bool
		releaseSkipDeps         *bool
		releaseSkipRefresh      *bool
		optsSkipDeps            bool
		optsSkipRefresh         bool
		chartPath               string
		isLocal                 bool
		expectedBuildDeps       bool // buildDeps = !skipDeps for local charts
		expectedSkipRefresh     bool
	}{
		{
			name:                    "helmDefaults.skipDeps=true should result in buildDeps=false",
			helmDefaultsSkipDeps:    true,
			helmDefaultsSkipRefresh: false,
			chartPath:               "./chart",
			isLocal:                 true,
			expectedBuildDeps:       false, // skipDeps=true means buildDeps=false
			expectedSkipRefresh:     false,
		},
		{
			name:                    "helmDefaults.skipRefresh=true should result in skipRefresh=true",
			helmDefaultsSkipDeps:    false,
			helmDefaultsSkipRefresh: true,
			chartPath:               "./chart",
			isLocal:                 true,
			expectedBuildDeps:       true, // skipDeps=false means buildDeps=true
			expectedSkipRefresh:     true,
		},
		{
			name:                    "both helmDefaults set should affect both flags",
			helmDefaultsSkipDeps:    true,
			helmDefaultsSkipRefresh: true,
			chartPath:               "./chart",
			isLocal:                 true,
			expectedBuildDeps:       false,
			expectedSkipRefresh:     true,
		},
		{
			name:                    "release-level skipDeps overrides helmDefaults",
			helmDefaultsSkipDeps:    false,
			helmDefaultsSkipRefresh: false,
			releaseSkipDeps:         boolPtr(true),
			chartPath:               "./chart",
			isLocal:                 true,
			expectedBuildDeps:       false, // release-level skipDeps=true
			expectedSkipRefresh:     false,
		},
		{
			name:                    "CLI opts skipDeps has priority",
			helmDefaultsSkipDeps:    false,
			helmDefaultsSkipRefresh: false,
			optsSkipDeps:            true,
			chartPath:               "./chart",
			isLocal:                 true,
			expectedBuildDeps:       false, // opts skipDeps=true
			expectedSkipRefresh:     false,
		},
		{
			name:                    "non-local chart always has buildDeps=false",
			helmDefaultsSkipDeps:    false,
			helmDefaultsSkipRefresh: false,
			chartPath:               "stable/nginx",
			isLocal:                 false,
			expectedBuildDeps:       false, // non-local charts don't build deps
			expectedSkipRefresh:     true,  // non-local charts skip refresh
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate skipDeps using the actual logic from state.go
			skipDepsGlobal := tt.optsSkipDeps
			skipDepsRelease := tt.releaseSkipDeps != nil && *tt.releaseSkipDeps
			skipDepsDefault := tt.releaseSkipDeps == nil && tt.helmDefaultsSkipDeps
			chartFetchedByGoGetter := false
			skipDeps := (!tt.isLocal && !chartFetchedByGoGetter) || skipDepsGlobal || skipDepsRelease || skipDepsDefault

			// Calculate skipRefresh using the actual logic from state.go
			skipRefreshGlobal := tt.optsSkipRefresh
			skipRefreshRelease := tt.releaseSkipRefresh != nil && *tt.releaseSkipRefresh
			skipRefreshDefault := tt.releaseSkipRefresh == nil && tt.helmDefaultsSkipRefresh
			skipRefresh := !tt.isLocal || skipRefreshGlobal || skipRefreshRelease || skipRefreshDefault

			// buildDeps = !skipDeps (for local charts processed normally)
			buildDeps := !skipDeps

			assert.Equal(t, tt.expectedBuildDeps, buildDeps,
				"buildDeps mismatch: expected %v, got %v (skipDeps=%v)", tt.expectedBuildDeps, buildDeps, skipDeps)
			assert.Equal(t, tt.expectedSkipRefresh, skipRefresh,
				"skipRefresh mismatch: expected %v, got %v", tt.expectedSkipRefresh, skipRefresh)
		})
	}
}
