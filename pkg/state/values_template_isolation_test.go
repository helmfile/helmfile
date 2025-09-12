package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/tmpl"
)

// TestValuesTemplateIsolation tests the fix for the helmfile values templating bug
// where changing the order of releases resulted in different values being used
func TestValuesTemplateIsolation(t *testing.T) {
	// Create test filesystem
	fs := &filesystem.FileSystem{
		Glob: func(pattern string) ([]string, error) {
			return nil, nil
		},
		ReadFile: func(filename string) ([]byte, error) {
			return nil, nil
		},
	}

	// Create test environment values
	envValues := map[string]any{
		"common": "shared-value",
		"foo": map[string]any{
			"name": "foo-chart",
		},
		"bar": map[string]any{
			"name": "bar-chart",
		},
	}

	st := &HelmState{
		fs:             fs,
		basePath:       "/tmp",
		RenderedValues: envValues,
	}

	// Create two releases that use valuesTemplate with mergeOverwrite
	fooRelease := &ReleaseSpec{
		Name:  "foo",
		Chart: "charts/foo",
		ValuesTemplate: []any{
			"{{ .Values | get \"foo\" (dict) | mergeOverwrite .Values | toYaml | nindent 8 }}",
		},
	}

	barRelease := &ReleaseSpec{
		Name:  "bar",
		Chart: "charts/bar",
		ValuesTemplate: []any{
			"{{ .Values | get \"bar\" (dict) | mergeOverwrite .Values | toYaml | nindent 8 }}",
		},
	}

	// Test: process foo first, then bar
	vals1 := st.Values()
	tmplData1, err := st.createReleaseTemplateData(fooRelease, vals1)
	require.NoError(t, err)

	renderer1 := tmpl.NewFileRenderer(st.fs, st.basePath, tmplData1)
	processedFooFirst, err := fooRelease.ExecuteTemplateExpressions(renderer1)
	require.NoError(t, err)

	vals2 := st.Values()
	tmplData2, err := st.createReleaseTemplateData(barRelease, vals2)
	require.NoError(t, err)

	renderer2 := tmpl.NewFileRenderer(st.fs, st.basePath, tmplData2)
	processedBarSecond, err := barRelease.ExecuteTemplateExpressions(renderer2)
	require.NoError(t, err)

	// Test: process bar first, then foo (reverse order)
	vals3 := st.Values()
	tmplData3, err := st.createReleaseTemplateData(barRelease, vals3)
	require.NoError(t, err)

	renderer3 := tmpl.NewFileRenderer(st.fs, st.basePath, tmplData3)
	processedBarFirst, err := barRelease.ExecuteTemplateExpressions(renderer3)
	require.NoError(t, err)

	vals4 := st.Values()
	tmplData4, err := st.createReleaseTemplateData(fooRelease, vals4)
	require.NoError(t, err)

	renderer4 := tmpl.NewFileRenderer(st.fs, st.basePath, tmplData4)
	processedFooSecond, err := fooRelease.ExecuteTemplateExpressions(renderer4)
	require.NoError(t, err)

	// Verify that the order doesn't matter - results should be consistent
	require.Equal(t, processedFooFirst.Values, processedFooSecond.Values,
		"foo release should produce same values regardless of processing order")
	require.Equal(t, processedBarSecond.Values, processedBarFirst.Values,
		"bar release should produce same values regardless of processing order")

	// Also verify that the original values are not modified
	originalVals := st.Values()
	require.Equal(t, envValues, originalVals,
		"original values should remain unchanged")
}
