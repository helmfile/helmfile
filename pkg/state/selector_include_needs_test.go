package state

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestIncludeNeedsVsIncludeTransitiveNeeds demonstrates the difference between
// --include-needs and --include-transitive-needs flags.
//
// Behavior Summary:
// 1. --include-needs: Includes only DIRECT dependencies (immediate needs) of selected releases
// 2. --include-transitive-needs: Includes ALL dependencies including transitive ones (needs of needs)
//
// Example dependency graph:
//
//	appA -> appB -> appC
//	appA -> appD
//
// When selecting appA with:
// - No flags: Only appA (fails if needs are not satisfied)
// - --include-needs: appA, appB, appD (only direct needs)
// - --include-transitive-needs: appA, appB, appC, appD (all needs including transitive)
func TestIncludeNeedsVsIncludeTransitiveNeeds(t *testing.T) {
	type testcase struct {
		name                   string
		selector               []string
		includeNeeds           bool
		includeTransitiveNeeds bool
		want                   []string
	}

	// Dependency graph:
	// appA needs [appB, appD]
	// appB needs [appC]
	// appC has no needs
	// appD has no needs
	// appE is independent (not in dependency chain)
	testcases := []testcase{
		{
			name:                   "no include flags - only selected release",
			selector:               []string{"name=appA"},
			includeNeeds:           false,
			includeTransitiveNeeds: false,
			want:                   []string{"appA"},
		},
		{
			name:                   "include-needs only - direct dependencies (appB, appD)",
			selector:               []string{"name=appA"},
			includeNeeds:           true,
			includeTransitiveNeeds: false,
			want:                   []string{"appA", "appB", "appD"},
		},
		{
			name:                   "include-transitive-needs - all dependencies including transitive (appB, appC, appD)",
			selector:               []string{"name=appA"},
			includeNeeds:           false, // Note: includeTransitiveNeeds implies includeNeeds
			includeTransitiveNeeds: true,
			want:                   []string{"appA", "appB", "appC", "appD"},
		},
		{
			name:                   "include-needs AND include-transitive-needs - same as include-transitive-needs alone",
			selector:               []string{"name=appA"},
			includeNeeds:           true,
			includeTransitiveNeeds: true,
			want:                   []string{"appA", "appB", "appC", "appD"},
		},
		{
			name:                   "include-needs on leaf release (appC) - no dependencies to include",
			selector:               []string{"name=appC"},
			includeNeeds:           true,
			includeTransitiveNeeds: false,
			want:                   []string{"appC"},
		},
		{
			name:                   "include-transitive-needs on middle release (appB) - includes appC",
			selector:               []string{"name=appB"},
			includeNeeds:           false,
			includeTransitiveNeeds: true,
			want:                   []string{"appB", "appC"},
		},
		{
			name:                   "include-needs on middle release (appB) - includes only appC (direct need)",
			selector:               []string{"name=appB"},
			includeNeeds:           true,
			includeTransitiveNeeds: false,
			want:                   []string{"appB", "appC"},
		},
	}

	example := []byte(`releases:
- name: appA
  namespace: default
  chart: stable/testchart
  needs:
    - appB
    - appD
- name: appB
  namespace: default
  chart: stable/testchart
  needs:
    - appC
- name: appC
  namespace: default
  chart: stable/testchart
- name: appD
  namespace: default
  chart: stable/testchart
- name: appE
  namespace: default
  chart: stable/testchart
`)

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			state := stateTestEnv{
				Files: map[string]string{
					"/helmfile.yaml": string(example),
				},
				WorkDir: "/",
			}.MustLoadState(t, "/helmfile.yaml", "default")

			var err error
			state.Selectors = tc.selector
			state.Releases, err = state.GetReleasesWithOverrides()
			if err != nil {
				t.Fatalf("GetReleasesWithOverrides failed: %v", err)
			}
			state.Releases = state.GetReleasesWithLabels()

			// GetSelectedReleases(includeNeeds, includeTransitiveNeeds)
			rs, err := state.GetSelectedReleases(tc.includeNeeds, tc.includeTransitiveNeeds)
			if err != nil {
				t.Fatalf("GetSelectedReleases failed: %v", err)
			}

			var got []string
			for _, r := range rs {
				got = append(got, r.Name)
			}

			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("unexpected releases: want (-), got (+):\n%s", d)
			}
		})
	}
}

// TestIncludeNeedsWithDeepTransitiveChain tests a deeper transitive dependency chain
// to ensure --include-needs only includes direct dependencies.
//
// Dependency graph: app1 -> app2 -> app3 -> app4
//
// With --include-needs on app1: should include app1, app2 (direct only)
// With --include-transitive-needs on app1: should include app1, app2, app3, app4
func TestIncludeNeedsWithDeepTransitiveChain(t *testing.T) {
	type testcase struct {
		name                   string
		selector               []string
		includeNeeds           bool
		includeTransitiveNeeds bool
		want                   []string
	}

	testcases := []testcase{
		{
			name:                   "include-needs on deep chain - direct dependency only",
			selector:               []string{"name=app1"},
			includeNeeds:           true,
			includeTransitiveNeeds: false,
			want:                   []string{"app1", "app2"},
		},
		{
			name:                   "include-transitive-needs on deep chain - all dependencies",
			selector:               []string{"name=app1"},
			includeNeeds:           false,
			includeTransitiveNeeds: true,
			want:                   []string{"app1", "app2", "app3", "app4"},
		},
		{
			name:                   "include-needs from middle of chain",
			selector:               []string{"name=app2"},
			includeNeeds:           true,
			includeTransitiveNeeds: false,
			want:                   []string{"app2", "app3"},
		},
		{
			name:                   "include-transitive-needs from middle of chain",
			selector:               []string{"name=app2"},
			includeNeeds:           false,
			includeTransitiveNeeds: true,
			want:                   []string{"app2", "app3", "app4"},
		},
	}

	example := []byte(`releases:
- name: app1
  namespace: default
  chart: stable/testchart
  needs:
    - app2
- name: app2
  namespace: default
  chart: stable/testchart
  needs:
    - app3
- name: app3
  namespace: default
  chart: stable/testchart
  needs:
    - app4
- name: app4
  namespace: default
  chart: stable/testchart
`)

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			state := stateTestEnv{
				Files: map[string]string{
					"/helmfile.yaml": string(example),
				},
				WorkDir: "/",
			}.MustLoadState(t, "/helmfile.yaml", "default")

			var err error
			state.Selectors = tc.selector
			state.Releases, err = state.GetReleasesWithOverrides()
			if err != nil {
				t.Fatalf("GetReleasesWithOverrides failed: %v", err)
			}
			state.Releases = state.GetReleasesWithLabels()

			rs, err := state.GetSelectedReleases(tc.includeNeeds, tc.includeTransitiveNeeds)
			if err != nil {
				t.Fatalf("GetSelectedReleases failed: %v", err)
			}

			var got []string
			for _, r := range rs {
				got = append(got, r.Name)
			}

			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("unexpected releases: want (-), got (+):\n%s", d)
			}
		})
	}
}

// TestIncludeNeedsWithMultipleDirectNeeds tests that --include-needs includes
// all direct needs but not transitive needs of those direct needs.
//
// Dependency graph:
//
//	frontend -> [backend-api, backend-worker]
//	backend-api -> database
//	backend-worker -> database
//	database -> cache
func TestIncludeNeedsWithMultipleDirectNeeds(t *testing.T) {
	type testcase struct {
		name                   string
		selector               []string
		includeNeeds           bool
		includeTransitiveNeeds bool
		want                   []string
	}

	testcases := []testcase{
		{
			name:                   "include-needs - direct needs only (backend-api, backend-worker)",
			selector:               []string{"name=frontend"},
			includeNeeds:           true,
			includeTransitiveNeeds: false,
			want:                   []string{"frontend", "backend-api", "backend-worker"},
		},
		{
			name:                   "include-transitive-needs - all needs (backend-api, backend-worker, database, cache)",
			selector:               []string{"name=frontend"},
			includeNeeds:           false,
			includeTransitiveNeeds: true,
			want:                   []string{"frontend", "backend-api", "backend-worker", "database", "cache"},
		},
	}

	example := []byte(`releases:
- name: frontend
  namespace: default
  chart: stable/testchart
  needs:
    - backend-api
    - backend-worker
- name: backend-api
  namespace: default
  chart: stable/testchart
  needs:
    - database
- name: backend-worker
  namespace: default
  chart: stable/testchart
  needs:
    - database
- name: database
  namespace: default
  chart: stable/testchart
  needs:
    - cache
- name: cache
  namespace: default
  chart: stable/testchart
`)

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			state := stateTestEnv{
				Files: map[string]string{
					"/helmfile.yaml": string(example),
				},
				WorkDir: "/",
			}.MustLoadState(t, "/helmfile.yaml", "default")

			var err error
			state.Selectors = tc.selector
			state.Releases, err = state.GetReleasesWithOverrides()
			if err != nil {
				t.Fatalf("GetReleasesWithOverrides failed: %v", err)
			}
			state.Releases = state.GetReleasesWithLabels()

			rs, err := state.GetSelectedReleases(tc.includeNeeds, tc.includeTransitiveNeeds)
			if err != nil {
				t.Fatalf("GetSelectedReleases failed: %v", err)
			}

			var got []string
			for _, r := range rs {
				got = append(got, r.Name)
			}

			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("unexpected releases: want (-), got (+):\n%s", d)
			}
		})
	}
}
