package state

import (
	"testing"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/stretchr/testify/require"
)

// TestIssue2182_ValuesTemplatingBugFix is an integration test that reproduces
// the exact scenario described in https://github.com/helmfile/helmfile/issues/2182
// and verifies that our fix works correctly.
func TestIssue2182_ValuesTemplatingBugFix(t *testing.T) {
	// Simulate the exact scenario from the issue:
	// environments:
	//   default:
	//     values:
	//       - values.yaml
	// ---
	// releases:
	//   - name: foo
	//     chart: charts/foo
	//     valuesTemplate:
	//       - {{ .Values | get "foo" (dict) | mergeOverwrite .Values | toYaml | nindent 8 }}
	//   - name: bar
	//     chart: charts/bar
	//     valuesTemplate:
	//       - {{ .Values | get "bar" (dict) | mergeOverwrite .Values | toYaml | nindent 8 }}

	// Create test filesystem
	fs := &filesystem.FileSystem{
		Glob: func(pattern string) ([]string, error) {
			return nil, nil
		},
		ReadFile: func(filename string) ([]byte, error) {
			return nil, nil
		},
	}

	// Simulate values.yaml content
	valuesYaml := map[string]any{
		"global":    "shared-config",
		"commonKey": "commonValue",
		"foo": map[string]any{
			"enabled":     true,
			"replicaCount": 2,
			"image":       "foo:1.0.0",
		},
		"bar": map[string]any{
			"enabled":     true,
			"replicaCount": 1,
			"image":       "bar:2.0.0",
		},
	}

	st := &HelmState{
		fs:             fs,
		basePath:       "/tmp",
		FilePath:       "helmfile.yaml",
		RenderedValues: valuesYaml,
	}

	// Define the releases as they would appear in helmfile.yaml
	fooRelease := &ReleaseSpec{
		Name:  "foo",
		Chart: "charts/foo",
		ValuesTemplate: []any{
			`{{ .Values | get "foo" (dict) | mergeOverwrite .Values | toYaml | nindent 8 }}`,
		},
	}

	barRelease := &ReleaseSpec{
		Name:  "bar",
		Chart: "charts/bar",
		ValuesTemplate: []any{
			`{{ .Values | get "bar" (dict) | mergeOverwrite .Values | toYaml | nindent 8 }}`,
		},
	}

	// Simulate ExecuteTemplates processing releases in order: foo then bar
	releases1 := []ReleaseSpec{*fooRelease, *barRelease}
	st.Releases = releases1

	result1, err := st.ExecuteTemplates()
	require.NoError(t, err, "ExecuteTemplates should succeed with foo then bar")

	// Simulate ExecuteTemplates processing releases in reverse order: bar then foo  
	releases2 := []ReleaseSpec{*barRelease, *fooRelease}
	st.Releases = releases2

	result2, err := st.ExecuteTemplates()
	require.NoError(t, err, "ExecuteTemplates should succeed with bar then foo")

	// Extract the processed releases from both executions
	fooRelease1 := result1.Releases[0] // foo from first execution (foo, bar)
	barRelease1 := result1.Releases[1] // bar from first execution (foo, bar)

	barRelease2 := result2.Releases[0] // bar from second execution (bar, foo)
	fooRelease2 := result2.Releases[1] // foo from second execution (bar, foo)

	// The critical assertion: Order should not matter!
	// Before the fix, the second release would see modified values from the first release
	require.Equal(t, fooRelease1.Values, fooRelease2.Values, 
		"foo release values should be identical regardless of processing order")
	require.Equal(t, barRelease1.Values, barRelease2.Values,
		"bar release values should be identical regardless of processing order")

	// Verify that each release gets the expected merged values
	// foo release should have foo-specific values merged into the root
	fooVals1 := fooRelease1.Values[0]
	require.Contains(t, fooVals1, "enabled")
	require.Contains(t, fooVals1, "replicaCount") 
	require.Contains(t, fooVals1, "image")
	require.Contains(t, fooVals1, "global")    // Should preserve global values
	require.Contains(t, fooVals1, "commonKey") // Should preserve common values

	// bar release should have bar-specific values merged into the root  
	barVals1 := barRelease1.Values[0]
	require.Contains(t, barVals1, "enabled")
	require.Contains(t, barVals1, "replicaCount")
	require.Contains(t, barVals1, "image") 
	require.Contains(t, barVals1, "global")    // Should preserve global values
	require.Contains(t, barVals1, "commonKey") // Should preserve common values

	// Verify that the original values were not mutated
	originalVals := st.Values()
	require.Equal(t, valuesYaml, originalVals, "original values should remain unchanged")

	t.Log("✅ Fix verified: Release order no longer affects values templating results")
}