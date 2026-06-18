package state

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDedupResolvedDependencies(t *testing.T) {
	tests := []struct {
		name     string
		input    []ResolvedChartDependency
		expected []ResolvedChartDependency
	}{
		{
			name: "no duplicates",
			input: []ResolvedChartDependency{
				{ChartName: "app-template", Repository: "https://example.com", Version: "4.6.2"},
				{ChartName: "redis", Repository: "https://charts.bitnami.com/bitnami", Version: "17.0.7"},
			},
			expected: []ResolvedChartDependency{
				{ChartName: "app-template", Repository: "https://example.com", Version: "4.6.2"},
				{ChartName: "redis", Repository: "https://charts.bitnami.com/bitnami", Version: "17.0.7"},
			},
		},
		{
			name: "duplicates removed",
			input: []ResolvedChartDependency{
				{ChartName: "app-template", Repository: "https://example.com", Version: "4.6.2"},
				{ChartName: "app-template", Repository: "https://example.com", Version: "4.6.2"},
			},
			expected: []ResolvedChartDependency{
				{ChartName: "app-template", Repository: "https://example.com", Version: "4.6.2"},
			},
		},
		{
			name: "same chart different versions kept",
			input: []ResolvedChartDependency{
				{ChartName: "app-template", Repository: "https://example.com", Version: "4.6.2"},
				{ChartName: "app-template", Repository: "https://example.com", Version: "4.6.1"},
			},
			expected: []ResolvedChartDependency{
				{ChartName: "app-template", Repository: "https://example.com", Version: "4.6.2"},
				{ChartName: "app-template", Repository: "https://example.com", Version: "4.6.1"},
			},
		},
		{
			name: "same chart different repos kept",
			input: []ResolvedChartDependency{
				{ChartName: "app-template", Repository: "https://example.com", Version: "4.6.2"},
				{ChartName: "app-template", Repository: "https://other.com", Version: "4.6.2"},
			},
			expected: []ResolvedChartDependency{
				{ChartName: "app-template", Repository: "https://example.com", Version: "4.6.2"},
				{ChartName: "app-template", Repository: "https://other.com", Version: "4.6.2"},
			},
		},
		{
			name:     "empty input",
			input:    []ResolvedChartDependency{},
			expected: []ResolvedChartDependency{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dedupResolvedDependencies(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetUnresolvedDependenciess(t *testing.T) {
	tests := []struct {
		name       string
		helmState  *HelmState
		expectfile string
		expectDeps *UnresolvedDependencies
	}{
		{
			name: "oci chart with path prefix and underscores (issue #954)",
			helmState: &HelmState{
				FilePath: "helmfile.yaml",
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: []ReleaseSpec{
						{
							Name:      "example",
							Chart:     "myrepo/path_with_underscores/example",
							Version:   "1.0.0",
							Namespace: "myns",
						},
						{
							Name:      "another",
							Chart:     "myrepo/another_path/chart",
							Version:   "2.0.0",
							Namespace: "myns",
						},
					},
					Repositories: []RepositorySpec{
						{
							Name: "myrepo",
							URL:  "harbor.custom.com",
							OCI:  true,
						},
					},
				},
			},
			expectfile: "helmfile",
			expectDeps: &UnresolvedDependencies{
				deps: map[string][]unresolvedChartDependency{
					"example": {
						{
							ChartName:         "example",
							Repository:        "oci://harbor.custom.com/path_with_underscores",
							VersionConstraint: "1.0.0",
							Alias:             "myns-example",
						},
					},
					"chart": {
						{
							ChartName:         "chart",
							Repository:        "oci://harbor.custom.com/another_path",
							VersionConstraint: "2.0.0",
							Alias:             "myns-another",
						},
					},
				},
			},
		},
		{
			name: "oci chart without path prefix (unchanged behavior)",
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
		{
			name: "duplicate charts are differentiated by alias",
			helmState: &HelmState{
				FilePath: "helmfile.yaml",
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: []ReleaseSpec{
						{
							Name:      "foo",
							Chart:     "myrepo/abc",
							Version:   "> 0.2.0",
							Namespace: "ns1",
						},
						{
							Name:      "bar",
							Chart:     "myrepo/abc",
							Version:   "0.1.0",
							Namespace: "ns2",
						},
						{
							Name:    "baz",
							Chart:   "myrepo/abc",
							Version: "0.3.0",
						},
					},
					Repositories: []RepositorySpec{
						{
							Name: "myrepo",
							URL:  "localhost:5000/aaa",
						},
					},
				},
			},
			expectfile: "helmfile",
			expectDeps: &UnresolvedDependencies{
				deps: map[string][]unresolvedChartDependency{
					"abc": {
						{
							ChartName:         "abc",
							Repository:        "localhost:5000/aaa",
							VersionConstraint: "> 0.2.0",
							Alias:             "ns1-foo",
						},
						{
							ChartName:         "abc",
							Repository:        "localhost:5000/aaa",
							VersionConstraint: "0.1.0",
							Alias:             "ns2-bar",
						},
						{
							ChartName:         "abc",
							Repository:        "localhost:5000/aaa",
							VersionConstraint: "0.3.0",
							Alias:             "-baz",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, ds := getUnresolvedDependenciess(tt.helmState)
			require.Equalf(t, tt.expectfile, f, "getUnresolvedDependenciess() expect file %s, got %s", tt.expectfile, f)
			require.Equalf(t, tt.expectDeps, ds, "getUnresolvedDependenciess() expect deps %v, got %v", tt.expectDeps, ds)
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

func TestOciDependencyChartName(t *testing.T) {
	tests := []struct {
		name  string
		chart string
		want  string
	}{
		{name: "simple chart name", chart: "example", want: "example"},
		{name: "path with underscores", chart: "path_with_underscores/example", want: "example"},
		{name: "deep nested path", chart: "deep/nested/chart", want: "chart"},
		{name: "single segment with underscore", chart: "my_chart", want: "my_chart"},
		{name: "path with hyphens", chart: "path-with-hyphens/example", want: "example"},
		{name: "empty string", chart: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ociDependencyChartName(tt.chart)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestOciDependencyRepoURL(t *testing.T) {
	tests := []struct {
		name       string
		chart      string
		ociBaseURL string
		want       string
	}{
		{
			name:       "simple chart (no path prefix)",
			chart:      "example",
			ociBaseURL: "oci://registry.example.com",
			want:       "oci://registry.example.com",
		},
		{
			name:       "chart with path prefix and underscores",
			chart:      "path_with_underscores/example",
			ociBaseURL: "oci://harbor.custom.com",
			want:       "oci://harbor.custom.com/path_with_underscores",
		},
		{
			name:       "deeply nested path",
			chart:      "deep/nested/chart",
			ociBaseURL: "oci://registry.example.com",
			want:       "oci://registry.example.com/deep/nested",
		},
		{
			name:       "base URL with trailing slash",
			chart:      "path/example",
			ociBaseURL: "oci://registry.example.com/",
			want:       "oci://registry.example.com/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ociDependencyRepoURL(tt.chart, tt.ociBaseURL)
			require.Equal(t, tt.want, got)
		})
	}
}
