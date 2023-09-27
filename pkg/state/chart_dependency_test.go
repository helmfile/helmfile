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
							Alias:             "foo",
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
