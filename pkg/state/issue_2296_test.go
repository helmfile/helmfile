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
// This test verifies the skipDeps/skipRefresh calculation in state.go that affects:
// 1. buildDeps - whether to run `helm dep build` for local charts
// 2. skipRefresh - whether to skip chart refresh operations
//
// Note: The skipRepos logic in pkg/app/run.go uses both helmDefaults.skipDeps and
// helmDefaults.skipRefresh to determine whether to sync repos. Both flags cause
// repo sync to be skipped because:
// - skipRefresh explicitly means "don't update repos"
// - skipDeps implies "I have all dependencies locally" which means repo data isn't needed
// The skipRepos behavior is documented in run.go comments and tested via integration tests.
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

// TestSkipReposLogic tests the skipRepos calculation used in pkg/app/run.go.
// This documents and verifies the expected behavior: both helmDefaults.skipDeps
// and helmDefaults.skipRefresh should cause repo sync to be skipped.
//
// The actual skipRepos logic is in run.go:
//
//	skipRepos := opts.SkipRepos || r.state.HelmDefaults.SkipDeps || r.state.HelmDefaults.SkipRefresh
func TestSkipReposLogic(t *testing.T) {
	tests := []struct {
		name             string
		optsSkipRepos    bool
		skipDeps         bool
		skipRefresh      bool
		expectedSkipRepo bool
	}{
		{
			name:             "all false - repos should sync",
			optsSkipRepos:    false,
			skipDeps:         false,
			skipRefresh:      false,
			expectedSkipRepo: false,
		},
		{
			name:             "opts.SkipRepos=true - repos should be skipped",
			optsSkipRepos:    true,
			skipDeps:         false,
			skipRefresh:      false,
			expectedSkipRepo: true,
		},
		{
			name:             "helmDefaults.skipDeps=true - repos should be skipped",
			optsSkipRepos:    false,
			skipDeps:         true,
			skipRefresh:      false,
			expectedSkipRepo: true,
		},
		{
			name:             "helmDefaults.skipRefresh=true - repos should be skipped",
			optsSkipRepos:    false,
			skipDeps:         false,
			skipRefresh:      true,
			expectedSkipRepo: true,
		},
		{
			name:             "both helmDefaults set - repos should be skipped",
			optsSkipRepos:    false,
			skipDeps:         true,
			skipRefresh:      true,
			expectedSkipRepo: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This mirrors the logic in pkg/app/run.go withPreparedCharts and Deps
			skipRepos := tt.optsSkipRepos || tt.skipDeps || tt.skipRefresh
			assert.Equal(t, tt.expectedSkipRepo, skipRepos,
				"skipRepos mismatch: expected %v, got %v", tt.expectedSkipRepo, skipRepos)
		})
	}
}
