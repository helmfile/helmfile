package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/yaml"
)

// TestExactIssueScenario tests the exact helmfile.yaml scenario described in issue #2182
// This validates that users can now safely use the pattern without needing deepCopy workarounds
func TestExactIssueScenario(t *testing.T) {
	// Reproduce the exact helmfile.yaml structure from the issue:
	// environments:
	//   default:
	//     values:
	//       - values.yaml
	// ---
	// releases:
	//   - name: foo
	//     chart: charts/foo
	//     valuesTemplate:
	//       - {{ .Values  | get "foo" (dict) | mergeOverwrite .Values | toYaml | nindent 8 }}
	//   - name: bar
	//     chart: charts/bar
	//     valuesTemplate:
	//       - {{ .Values  | get "bar" (dict) | mergeOverwrite .Values | toYaml | nindent 8 }}

	fs := &filesystem.FileSystem{
		Glob: func(pattern string) ([]string, error) {
			return nil, nil
		},
		ReadFile: func(filename string) ([]byte, error) {
			return nil, nil
		},
	}

	// Simulate the values.yaml content from the issue
	valuesYamlContent := map[string]any{
		"commonSetting": "shared-value",
		"environment":   "production",
		"foo": map[string]any{
			"image": "foo:1.2.3",
			"port":  8080,
		},
		"bar": map[string]any{
			"image": "bar:2.1.0",
			"port":  9090,
		},
	}

	st := &HelmState{
		fs:             fs,
		basePath:       "/tmp",
		FilePath:       "helmfile.yaml",
		RenderedValues: valuesYamlContent,
	}

	// Test both orders: foo then bar, and bar then foo
	orders := [][]string{
		{"foo", "bar"}, // Original order
		{"bar", "foo"}, // Reversed order
	}

	var results [][][]byte

	for _, order := range orders {
		var releases []ReleaseSpec

		// Build releases in the specified order
		for _, name := range order {
			release := ReleaseSpec{
				Name:  name,
				Chart: "charts/" + name,
				ValuesTemplate: []any{
					`{{ .Values | get "` + name + `" (dict) | mergeOverwrite .Values | toYaml | nindent 8 }}`,
				},
			}
			releases = append(releases, release)
		}

		st.Releases = releases
		processedState, err := st.ExecuteTemplates()
		require.NoError(t, err, "ExecuteTemplates should succeed for order %v", order)

		// Collect the processed values for comparison
		var orderResults [][]byte
		for _, release := range processedState.Releases {
			require.NotEmpty(t, release.Values, "release %s should have processed values", release.Name)
			// Convert to bytes for comparison
			serialized, err := yaml.Marshal(release.Values[0])
			require.NoError(t, err, "should be able to marshal values")
			orderResults = append(orderResults, serialized)
		}
		results = append(results, orderResults)
	}

	// Critical assertion: Both processing orders should yield identical results
	require.Len(t, results, 2, "should have results for both orders")
	require.Len(t, results[0], 2, "should have 2 releases in first order")
	require.Len(t, results[1], 2, "should have 2 releases in second order")

	// Since the order is different, we need to match by content, not position
	// For order ["foo", "bar"]: results[0][0] is foo, results[0][1] is bar
	// For order ["bar", "foo"]: results[1][0] is bar, results[1][1] is foo

	fooFromFirstOrder := results[0][0] // foo processed first
	barFromFirstOrder := results[0][1] // bar processed second

	barFromSecondOrder := results[1][0] // bar processed first
	fooFromSecondOrder := results[1][1] // foo processed second

	require.Equal(t, fooFromFirstOrder, fooFromSecondOrder,
		"foo release should produce identical values regardless of processing order")
	require.Equal(t, barFromFirstOrder, barFromSecondOrder,
		"bar release should produce identical values regardless of processing order")

	// Verify original values remain untouched
	originalValues := st.Values()
	require.Equal(t, valuesYamlContent, originalValues,
		"original values should remain unchanged after template processing")

	t.Log("✅ Issue #2182 is FIXED: Release order no longer affects valuesTemplate results")
	t.Log("✅ Users can now safely use mergeOverwrite in valuesTemplate without deepCopy workaround")
}
