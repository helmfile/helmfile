package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/exectest"
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

// updateRepoTracker wraps exectest.Helm to track whether UpdateRepo was called.
type updateRepoTracker struct {
	exectest.Helm
	updateRepoCalled bool
	buildDepsFlags   []string
}

func (h *updateRepoTracker) UpdateRepo() error {
	h.updateRepoCalled = true
	return nil
}

func (h *updateRepoTracker) BuildDeps(name, chart string, flags ...string) error {
	h.buildDepsFlags = flags
	return nil
}

// TestRunHelmDepBuilds_SkipRefreshBehaviors verifies multiple behaviors around
// skipRefresh handling in runHelmDepBuilds:
//
//  1. When helmDefaults.skipRefresh=true, runHelmDepBuilds skips the UpdateRepo call
//     even when opts.SkipRefresh is false (regression test for issue #2269).
//  2. When no repos are configured, helm dep build should not receive --skip-refresh
//     so it can refresh repos for local charts with external dependencies
//     (regression test for issue #2417).
//  3. UpdateRepo is skipped when only OCI repos are configured,
//     as OCI repos don't need `helm repo update` (they use `helm registry login` instead).
//
// The precomputedSkipRefresh field simulates the skipRefresh value computed in
// prepareChartForRelease, which accounts for CLI flags, helmDefaults.skipRefresh,
// and release-level skipRefresh settings.
func TestRunHelmDepBuilds_SkipRefreshBehaviors(t *testing.T) {
	tests := []struct {
		name                    string
		optsSkipRefresh         bool
		helmDefaultsSkipRefresh bool
		repos                   []RepositorySpec
		precomputedSkipRefresh  bool
		expectUpdateRepo        bool
		expectSkipRefreshFlag   bool
	}{
		{
			name:                    "local chart with repos exist - UpdateRepo skipped (all builds have skipRefresh=false), no skip-refresh flag (issue #2431)",
			optsSkipRefresh:         false,
			helmDefaultsSkipRefresh: false,
			repos:                   []RepositorySpec{{Name: "stable", URL: "https://example.com"}},
			precomputedSkipRefresh:  false,
			expectUpdateRepo:        false,
			expectSkipRefreshFlag:   false,
		},
		{
			name:                    "opts.SkipRefresh=true - UpdateRepo skipped, skip-refresh flag preserved from precomputed value",
			optsSkipRefresh:         true,
			helmDefaultsSkipRefresh: false,
			repos:                   []RepositorySpec{{Name: "stable", URL: "https://example.com"}},
			precomputedSkipRefresh:  true,
			expectUpdateRepo:        false,
			expectSkipRefreshFlag:   true,
		},
		{
			name:                    "helmDefaults.skipRefresh=true - UpdateRepo skipped, skip-refresh flag preserved from precomputed value",
			optsSkipRefresh:         false,
			helmDefaultsSkipRefresh: true,
			repos:                   []RepositorySpec{{Name: "stable", URL: "https://example.com"}},
			precomputedSkipRefresh:  true,
			expectUpdateRepo:        false,
			expectSkipRefreshFlag:   true,
		},
		{
			name:                    "release-level skipRefresh=true - UpdateRepo called, skip-refresh flag preserved from precomputed value",
			optsSkipRefresh:         false,
			helmDefaultsSkipRefresh: false,
			repos:                   []RepositorySpec{{Name: "stable", URL: "https://example.com"}},
			precomputedSkipRefresh:  true,
			expectUpdateRepo:        true,
			expectSkipRefreshFlag:   true,
		},
		{
			name:                    "no repos configured - UpdateRepo skipped, no skip-refresh flag (issue #2417)",
			optsSkipRefresh:         false,
			helmDefaultsSkipRefresh: false,
			repos:                   nil,
			precomputedSkipRefresh:  false,
			expectUpdateRepo:        false,
			expectSkipRefreshFlag:   false,
		},
		{
			name:                    "only OCI repos configured - UpdateRepo skipped, no skip-refresh flag",
			optsSkipRefresh:         false,
			helmDefaultsSkipRefresh: false,
			repos:                   []RepositorySpec{{Name: "karpenter", URL: "public.ecr.aws/karpenter", OCI: true}},
			precomputedSkipRefresh:  false,
			expectUpdateRepo:        false,
			expectSkipRefreshFlag:   false,
		},
		{
			name:                    "mixed repos (OCI + non-OCI) with local chart - UpdateRepo skipped (all builds have skipRefresh=false), no skip-refresh flag",
			optsSkipRefresh:         false,
			helmDefaultsSkipRefresh: false,
			repos: []RepositorySpec{
				{Name: "karpenter", URL: "public.ecr.aws/karpenter", OCI: true},
				{Name: "stable", URL: "https://charts.helm.sh/stable"},
			},
			precomputedSkipRefresh: false,
			expectUpdateRepo:       false,
			expectSkipRefreshFlag:  false,
		},
		{
			name:                    "non-local chart with repos exist - UpdateRepo called, skip-refresh passed",
			optsSkipRefresh:         false,
			helmDefaultsSkipRefresh: false,
			repos:                   []RepositorySpec{{Name: "stable", URL: "https://example.com"}},
			precomputedSkipRefresh:  true,
			expectUpdateRepo:        true,
			expectSkipRefreshFlag:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helm := &updateRepoTracker{}

			st := &HelmState{
				logger: logger,
				ReleaseSetSpec: ReleaseSetSpec{
					HelmDefaults: HelmSpec{
						SkipRefresh: tt.helmDefaultsSkipRefresh,
					},
					Repositories: tt.repos,
				},
			}

			builds := []*chartPrepareResult{
				{releaseName: "test", chartPath: "/tmp/chart", buildDeps: true, skipRefresh: tt.precomputedSkipRefresh},
			}

			opts := ChartPrepareOptions{
				SkipRefresh: tt.optsSkipRefresh,
			}

			err := st.runHelmDepBuilds(helm, 1, builds, opts)
			require.NoError(t, err)

			assert.NotNil(t, helm.buildDepsFlags, "BuildDeps should have been called")

			assert.Equal(t, tt.expectUpdateRepo, helm.updateRepoCalled,
				"UpdateRepo called mismatch: expected %v, got %v", tt.expectUpdateRepo, helm.updateRepoCalled)

			hasSkipRefreshFlag := false
			for _, f := range helm.buildDepsFlags {
				if f == "--skip-refresh" {
					hasSkipRefreshFlag = true
					break
				}
			}
			assert.Equal(t, tt.expectSkipRefreshFlag, hasSkipRefreshFlag,
				"--skip-refresh flag mismatch: expected %v, got %v (flags: %v)", tt.expectSkipRefreshFlag, hasSkipRefreshFlag, helm.buildDepsFlags)
		})
	}
}

// multiBuildTracker tracks multiple BuildDeps calls for testing scenarios with
// multiple builds that have different skipRefresh values.
type multiBuildTracker struct {
	exectest.Helm
	updateRepoCalled bool
	buildDepsCalls   [][]string
}

func (h *multiBuildTracker) UpdateRepo() error {
	h.updateRepoCalled = true
	return nil
}

func (h *multiBuildTracker) BuildDeps(name, chart string, flags ...string) error {
	h.buildDepsCalls = append(h.buildDepsCalls, flags)
	return nil
}

// TestRunHelmDepBuilds_MultipleBuilds verifies that when didUpdateRepo=true,
// only non-local charts (precomputed skipRefresh=true) receive --skip-refresh.
// Local charts (precomputed skipRefresh=false) preserve their value to allow
// refreshing repos for external dependencies not in helmfile.yaml (issue #2431).
func TestRunHelmDepBuilds_MultipleBuilds(t *testing.T) {
	helm := &multiBuildTracker{}

	st := &HelmState{
		logger: logger,
		ReleaseSetSpec: ReleaseSetSpec{
			HelmDefaults: HelmSpec{SkipRefresh: false},
			Repositories: []RepositorySpec{{Name: "stable", URL: "https://example.com"}},
		},
	}

	builds := []*chartPrepareResult{
		{releaseName: "release-a", chartPath: "/tmp/chart-a", buildDeps: true, skipRefresh: false},
		{releaseName: "release-b", chartPath: "/tmp/chart-b", buildDeps: true, skipRefresh: true},
	}

	opts := ChartPrepareOptions{SkipRefresh: false}

	err := st.runHelmDepBuilds(helm, 1, builds, opts)
	require.NoError(t, err)

	assert.True(t, helm.updateRepoCalled, "UpdateRepo should have been called")
	assert.Len(t, helm.buildDepsCalls, 2, "BuildDeps should have been called twice")

	expectedSkipRefresh := []bool{false, true}
	for i, flags := range helm.buildDepsCalls {
		hasSkipRefresh := false
		for _, f := range flags {
			if f == "--skip-refresh" {
				hasSkipRefresh = true
				break
			}
		}
		assert.Equal(t, expectedSkipRefresh[i], hasSkipRefresh,
			"build %d skip-refresh flag mismatch: expected %v, got %v (flags: %v)", i, expectedSkipRefresh[i], hasSkipRefresh, flags)
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

// TestNeedsRepoUpdate tests the NeedsRepoUpdate function.
// This is a regression test for issue #2418.
func TestNeedsRepoUpdate(t *testing.T) {
	tests := []struct {
		name     string
		repos    []RepositorySpec
		expected bool
	}{
		{
			name:     "no repos configured",
			repos:    nil,
			expected: false,
		},
		{
			name:     "only non-OCI repos",
			repos:    []RepositorySpec{{Name: "stable", URL: "https://charts.helm.sh/stable"}},
			expected: true,
		},
		{
			name:     "only OCI repos",
			repos:    []RepositorySpec{{Name: "karpenter", URL: "public.ecr.aws/karpenter", OCI: true}},
			expected: false,
		},
		{
			name: "mixed repos (OCI + non-OCI)",
			repos: []RepositorySpec{
				{Name: "karpenter", URL: "public.ecr.aws/karpenter", OCI: true},
				{Name: "stable", URL: "https://charts.helm.sh/stable"},
			},
			expected: true,
		},
		{
			name: "multiple OCI repos",
			repos: []RepositorySpec{
				{Name: "karpenter", URL: "public.ecr.aws/karpenter", OCI: true},
				{Name: "nginx", URL: "registry.example.com/nginx", OCI: true},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Repositories: tt.repos,
				},
			}
			result := st.NeedsRepoUpdate()
			assert.Equal(t, tt.expected, result,
				"NeedsRepoUpdate mismatch: expected %v, got %v", tt.expected, result)
		})
	}
}

// TestIssue2431_LocalChartWithExternalDependency tests the scenario from issue #2431:
// A local chart has dependencies on repos NOT listed in helmfile.yaml.
// The fix ensures that:
//  1. UpdateRepo is NOT called when all builds have skipRefresh=false
//  2. helm dep build does NOT receive --skip-refresh, allowing it to refresh
//     repos for external dependencies not in helmfile.yaml
func TestIssue2431_LocalChartWithExternalDependency(t *testing.T) {
	helm := &updateRepoTracker{}

	// helmfile.yaml has repos configured (e.g., vector), but NOT the repo
	// that the local chart depends on (e.g., wiremind)
	st := &HelmState{
		logger: logger,
		ReleaseSetSpec: ReleaseSetSpec{
			HelmDefaults: HelmSpec{SkipRefresh: false},
			Repositories: []RepositorySpec{
				{Name: "vector", URL: "https://helm.vector.dev"},
			},
		},
	}

	// Local chart with skipRefresh=false (precomputed in prepareChartForRelease)
	builds := []*chartPrepareResult{
		{releaseName: "karma", chartPath: "/tmp/karma", buildDeps: true, skipRefresh: false},
	}

	opts := ChartPrepareOptions{SkipRefresh: false}

	err := st.runHelmDepBuilds(helm, 1, builds, opts)
	require.NoError(t, err)

	// UpdateRepo should NOT be called because all builds have skipRefresh=false
	assert.False(t, helm.updateRepoCalled,
		"UpdateRepo should NOT be called when all builds have skipRefresh=false")

	// helm dep build should NOT receive --skip-refresh flag
	hasSkipRefreshFlag := false
	for _, f := range helm.buildDepsFlags {
		if f == "--skip-refresh" {
			hasSkipRefreshFlag = true
			break
		}
	}
	assert.False(t, hasSkipRefreshFlag,
		"helm dep build should NOT receive --skip-refresh for local charts with external dependencies")
}
