package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFilterReleasesForBuild is a regression test for issue #1859.
//
// Background: `helmfile build` is a read-only inspection command that outputs
// the helmfile state. It runs with SkipRepos: true and SkipDeps: true, meaning
// repositories are NOT synced before chart preparation. However, releases with
// dependencies, jsonPatches, strategicMergePatches, transformers, or
// forceNamespace trigger chartify, which runs `helm fetch`/`helm template` and
// requires repos to be synced — causing "repo <repo> not found" errors.
//
// filterReleasesForBuild excludes such releases from chart preparation during
// build so that the command succeeds without network access.
func TestFilterReleasesForBuild(t *testing.T) {
	tests := []struct {
		name     string
		releases []ReleaseSpec
		want     []string
	}{
		{
			name: "plain release is kept",
			releases: []ReleaseSpec{
				{Name: "plain", Chart: "examples/hello"},
			},
			want: []string{"plain"},
		},
		{
			name: "release with dependencies is filtered out",
			releases: []ReleaseSpec{
				{Name: "with-deps", Chart: "examples/hello", Dependencies: []Dependency{
					{Chart: "examples/hello", Version: "0.1.0"},
				}},
			},
			want: []string{},
		},
		{
			name: "release with jsonPatches is filtered out",
			releases: []ReleaseSpec{
				{Name: "with-jsonpatches", Chart: "examples/hello", JSONPatches: []any{map[string]any{"op": "add"}}},
			},
			want: []string{},
		},
		{
			name: "release with strategicMergePatches is filtered out",
			releases: []ReleaseSpec{
				{Name: "with-smp", Chart: "examples/hello", StrategicMergePatches: []any{"patch.yaml"}},
			},
			want: []string{},
		},
		{
			name: "release with transformers is filtered out",
			releases: []ReleaseSpec{
				{Name: "with-transformers", Chart: "examples/hello", Transformers: []any{"labels.yaml"}},
			},
			want: []string{},
		},
		{
			name: "release with forceNamespace is filtered out",
			releases: []ReleaseSpec{
				{Name: "with-fn", Chart: "examples/hello", ForceNamespace: "my-ns"},
			},
			want: []string{},
		},
		{
			name: "mix of plain and filtered releases",
			releases: []ReleaseSpec{
				{Name: "plain", Chart: "examples/hello"},
				{Name: "with-deps", Chart: "examples/hello", Dependencies: []Dependency{
					{Chart: "examples/hello", Version: "0.1.0"},
				}},
				{Name: "with-fn", Chart: "examples/hello", ForceNamespace: "my-ns"},
				{Name: "also-plain", Chart: "examples/goodbye"},
			},
			want: []string{"plain", "also-plain"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterReleasesForBuild(tt.releases)
			names := make([]string, 0, len(got))
			for _, r := range got {
				names = append(names, r.Name)
			}
			assert.Equal(t, tt.want, names)
		})
	}
}
