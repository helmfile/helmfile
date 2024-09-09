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
							Name:      "foo",
							Chart:     "chartsa/abc",
							Version:   "0.1.0",
							Namespace: "ns1",
						},
						{
							Name:      "empty",
							Chart:     "chartsb/empty",
							Namespace: "ns2",
						},
						{
							Name:  "empty",
							Chart: "chartsb/empty",
						},
					},
					Repositories: []RepositorySpec{
						{
							Name: "chartsa",
							URL:  "localhost:5000/aaa",
							OCI:  true,
						},
						{
							Name: "chartsb",
							URL:  "localhost:5000/bbb",
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
							Alias:             "ns1-foo",
						},
					},
					"empty": {
						{
							ChartName:  "empty",
							Repository: "localhost:5000/bbb",
							Alias:      "ns2-empty",
						},
						{
							ChartName:  "empty",
							Repository: "localhost:5000/bbb",
							Alias:      "-empty",
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
				Alias:             "abc-abc",
			},
			deps: map[string][]unresolvedChartDependency{
				"abc": {
					{
						ChartName:         "abc",
						Repository:        "oci://localhost:5000/aaa",
						VersionConstraint: "0.1.0",
						Alias:             "abc-abc",
					},
				},
			},
			expected: true,
		},
		{
			name: "existing dependency but empty item",
			dep: unresolvedChartDependency{
				ChartName:         "abc",
				Repository:        "oci://localhost:5000/aaa",
				VersionConstraint: "0.1.0",
				Alias:             "abc-abc",
			},
			deps: map[string][]unresolvedChartDependency{
				"abc": {
					{
						ChartName:  "abc",
						Repository: "oci://localhost:5000/aaa",
						Alias:      "abc-abc",
					},
				},
			},
			expected: false,
		},
		{
			name: "different chart version",
			dep: unresolvedChartDependency{
				ChartName:         "abc",
				Repository:        "oci://localhost:5000/aaa",
				VersionConstraint: "0.1.0",
				Alias:             "abc-abc",
			},
			deps: map[string][]unresolvedChartDependency{
				"abc": {
					{
						ChartName:         "abc",
						Repository:        "oci://localhost:5000/aaa",
						VersionConstraint: "0.1.1",
						Alias:             "abc-abc",
					},
				},
			},
			expected: false,
		},
		{
			name: "existing dependency with empty item",
			dep: unresolvedChartDependency{
				ChartName:         "ghi",
				Repository:        "oci://localhost:5000/aaa",
				VersionConstraint: "0.1.0",
				Alias:             "ghi-ghi",
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
				Alias:             "def-def",
			},
			deps: map[string][]unresolvedChartDependency{
				"abc": {
					{
						ChartName:         "abc",
						Repository:        "oci://localhost:5000/aaa",
						VersionConstraint: "0.1.0",
						Alias:             "abc-abc",
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

func TestChartDependenciesAlias(t *testing.T) {
	type testCase struct {
		releaseName string
		namespace   string
		expected    string
	}

	testCases := []testCase{
		{"release1", "n1", "n1-release1"},
		{"release2", "n2", "n2-release2"},
		{"empty", "", "-empty"},
	}

	for _, tc := range testCases {
		result := chartDependenciesAlias(tc.namespace, tc.releaseName)
		if result != tc.expected {
			t.Errorf("Expected %s, but got %s", tc.expected, result)
		}
	}
}
