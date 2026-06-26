package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestResolvedDependencies_Get(t *testing.T) {
	tests := []struct {
		name              string
		deps              map[string][]ResolvedChartDependency
		chart             string
		versionConstraint string
		allowMismatch     bool
		wantVersion       string
		wantErr           bool
	}{
		{
			name: "satisfying version returns it",
			deps: map[string][]ResolvedChartDependency{
				"mongodb": {{ChartName: "mongodb", Version: "13.10.3"}},
			},
			chart:             "mongodb",
			versionConstraint: "13.10.3",
			wantVersion:       "13.10.3",
		},
		{
			name: "wildcard constraint matches locked version",
			deps: map[string][]ResolvedChartDependency{
				"mongodb": {{ChartName: "mongodb", Version: "13.10.2"}},
			},
			chart:             "mongodb",
			versionConstraint: "*",
			wantVersion:       "13.10.2",
		},
		{
			// Default (strict) behavior: a lock file whose version does not satisfy
			// the helmfile.yaml constraint fails loudly instead of silently
			// deploying the wrong chart version (issue #870).
			name: "constraint mismatch errors by default (strict)",
			deps: map[string][]ResolvedChartDependency{
				"mongodb": {{ChartName: "mongodb", Version: "13.10.2"}},
			},
			chart:             "mongodb",
			versionConstraint: "13.10.3",
			wantErr:           true,
		},
		{
			// Opt-in behavior: with allowLockedVersionMismatch the highest locked
			// version is used even though it does not satisfy the constraint.
			name: "constraint mismatch falls back to highest locked version when allowed",
			deps: map[string][]ResolvedChartDependency{
				"mongodb": {
					{ChartName: "mongodb", Version: "13.10.1"},
					{ChartName: "mongodb", Version: "13.10.3"},
					{ChartName: "mongodb", Version: "13.10.2"},
				},
			},
			chart:             "mongodb",
			versionConstraint: "14.0.0",
			allowMismatch:     true,
			wantVersion:       "13.10.3",
		},
		{
			name: "range constraint picks satisfying locked version among several",
			deps: map[string][]ResolvedChartDependency{
				"envoy": {
					{ChartName: "envoy", Version: "1.4.0"},
					{ChartName: "envoy", Version: "1.5.0"},
				},
			},
			chart:             "envoy",
			versionConstraint: ">=1.5.0",
			wantVersion:       "1.5.0",
		},
		{
			name:              "unknown chart errors",
			deps:              map[string][]ResolvedChartDependency{},
			chart:             "missing",
			versionConstraint: "*",
			wantErr:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &ResolvedDependencies{deps: tt.deps, allowMismatch: tt.allowMismatch}
			got, err := d.Get(tt.chart, tt.versionConstraint)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantVersion, got)
		})
	}
}

// TestResolvedDependencies_Get_MismatchWarning asserts that, when mismatch
// fallback is allowed, a warning describing the drift is emitted.
func TestResolvedDependencies_Get_MismatchWarning(t *testing.T) {
	core, obs := observer.New(zapcore.WarnLevel)
	d := &ResolvedDependencies{
		deps: map[string][]ResolvedChartDependency{
			"mongodb": {{ChartName: "mongodb", Version: "13.10.2"}},
		},
		logger:        zap.New(core).Sugar(),
		allowMismatch: true,
	}

	got, err := d.Get("mongodb", "13.10.3")
	require.NoError(t, err)
	assert.Equal(t, "13.10.2", got)

	warns := obs.FilterMessageSnippet("does not satisfy").All()
	require.Len(t, warns, 1, "expected a single drift warning")
	assert.Equal(t, zapcore.WarnLevel, warns[0].Level)
	assert.Contains(t, warns[0].Message, "mongodb", "warning should name the chart")
}

// TestResolvedDependencies_Get_StrictNoWarning asserts the default (strict)
// path emits no fallback warning, only an error.
func TestResolvedDependencies_Get_StrictNoWarning(t *testing.T) {
	core, obs := observer.New(zapcore.DebugLevel)
	d := &ResolvedDependencies{
		deps: map[string][]ResolvedChartDependency{
			"mongodb": {{ChartName: "mongodb", Version: "13.10.2"}},
		},
		logger:        zap.New(core).Sugar(),
		allowMismatch: false,
	}

	_, err := d.Get("mongodb", "13.10.3")
	require.Error(t, err)
	assert.Empty(t, obs.All(), "strict path must not emit a fallback warning")
}

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
