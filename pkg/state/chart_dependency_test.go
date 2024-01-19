package state

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetUnresolvedDependenciess(t *testing.T) {
	tests := []struct {
		name       string
		helmState  *HelmState
		wantErr    bool
		expectfile string
		expectDeps *UnresolvedDependencies
	}{
		{
			name: "oci chart",
			helmState: &HelmState{
				FilePath: "helmfile.yaml",
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: []ReleaseSpec{
						{
							Name:    "foo",
							Chart:   "charts/abc",
							Version: "0.1.0",
						},
					},
					Repositories: []RepositorySpec{
						{
							Name: "charts",
							URL:  "localhost:5000/aaa",
							OCI:  true,
						},
					},
				},
			},
			wantErr:    false,
			expectfile: "helmfile",
			expectDeps: &UnresolvedDependencies{
				deps: map[string][]unresolvedChartDependency{
					"abc": {
						{
							ChartName:         "abc",
							Repository:        "oci://localhost:5000/aaa",
							VersionConstraint: "0.1.0",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, ds, err := getUnresolvedDependenciess(tt.helmState)
			if tt.wantErr {
				require.Error(t, err, "getUnresolvedDependenciess() error = nil, wantErr")
			} else {
				require.NoErrorf(t, err, "getUnresolvedDependenciess() want no error, got %v", err)
			}
			require.Equalf(t, tt.expectfile, f, "getUnresolvedDependenciess() expect file %s, got %s", tt.expectfile, f)
			require.Equalf(t, tt.expectDeps, ds, "getUnresolvedDependenciess() expect deps %v, got %v", tt.expectDeps, ds)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		dep      unresolvedChartDependency
		deps     map[string][]unresolvedChartDependency
		expected bool
	}{
		{
			name: "existing dependency with right item",
			dep: unresolvedChartDependency{
				ChartName:         "abc",
				Repository:        "oci://localhost:5000/aaa",
				VersionConstraint: "0.1.0",
			},
			deps: map[string][]unresolvedChartDependency{
				"abc": {
					{
						ChartName:         "abc",
						Repository:        "oci://localhost:5000/aaa",
						VersionConstraint: "0.1.0",
					},
				},
			},
			expected: true,
		},
		{
			name: "existing dependency with empty item",
			dep: unresolvedChartDependency{
				ChartName:         "ghi",
				Repository:        "oci://localhost:5000/aaa",
				VersionConstraint: "0.1.0",
			},
			deps: map[string][]unresolvedChartDependency{
				"ghi": {},
			},
			expected: false,
		},
		{
			name: "non-existing dependency",
			dep: unresolvedChartDependency{
				ChartName:         "def",
				Repository:        "oci://localhost:5000/bbb",
				VersionConstraint: "0.2.0",
			},
			deps: map[string][]unresolvedChartDependency{
				"abc": {
					{
						ChartName:         "abc",
						Repository:        "oci://localhost:5000/aaa",
						VersionConstraint: "0.1.0",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &UnresolvedDependencies{
				deps: tt.deps,
			}
			actual := d.contains(tt.dep)
			require.Equal(t, tt.expected, actual)
		})
	}
}
