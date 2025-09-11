package state

import (
	"testing"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/stretchr/testify/require"
)

// TestValuesMutationFix reproduces and tests the fix for the issue described in
// https://github.com/helmfile/helmfile/issues/2182
// where mergeOverwrite modifies the global .Values object instead of creating a local copy
func TestValuesMutationFix(t *testing.T) {
	// Create test filesystem with no files
	fs := &filesystem.FileSystem{
		Glob: func(pattern string) ([]string, error) {
			return nil, nil
		},
		ReadFile: func(filename string) ([]byte, error) {
			return nil, nil
		},
	}

	st := &HelmState{
		fs:       fs,
		basePath: "/tmp",
		RenderedValues: map[string]any{
			"common": "value",
			"foo": map[string]any{
				"specific": "foo-value",
			},
			"bar": map[string]any{
				"specific": "bar-value",
			},
		},
	}

	release := &ReleaseSpec{
		Name: "test-release",
		Chart: "test/chart",
	}

	// Create template data twice to simulate two different releases
	vals1 := st.Values()
	tmplData1, err := st.createReleaseTemplateData(release, vals1)
	require.NoError(t, err, "first createReleaseTemplateData should not fail")

	vals2 := st.Values()
	tmplData2, err := st.createReleaseTemplateData(release, vals2)
	require.NoError(t, err, "second createReleaseTemplateData should not fail")

	// Verify that both template data have the same initial values
	require.Equal(t, tmplData1.Values, tmplData2.Values, "both template data should start with identical values")

	// Simulate mergeOverwrite operation on first template data
	// This should not affect the second template data after our fix
	fooSection, ok := tmplData1.Values["foo"].(map[string]any)
	require.True(t, ok, "foo section should be a map")
	
	// Manually perform what mergeOverwrite would do - add values from foo section to the root
	for k, v := range fooSection {
		tmplData1.Values[k] = v
	}

	// Verify that the modification only affected tmplData1, not tmplData2
	_, hasSpecificInTmpl1 := tmplData1.Values["specific"]
	_, hasSpecificInTmpl2 := tmplData2.Values["specific"]

	require.True(t, hasSpecificInTmpl1, "tmplData1 should have the merged 'specific' key")
	require.False(t, hasSpecificInTmpl2, "tmplData2 should NOT have the merged 'specific' key (values should be isolated)")

	// Also verify that the original values are not affected
	originalVals := st.Values()
	_, hasSpecificInOriginal := originalVals["specific"]
	require.False(t, hasSpecificInOriginal, "original Values should NOT be affected")
}