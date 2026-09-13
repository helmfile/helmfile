package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/environment"
)

func TestNewEnvironmentTemplateData_EnvironmentValuesExcludesDefaults_Issue2527(t *testing.T) {
	env := environment.Environment{
		Name: "myenv",
		Defaults: map[string]any{
			"helmDefaults": map[string]any{
				"atomic":  true,
				"wait":    true,
				"timeout": 300,
			},
		},
		Values: map[string]any{
			"appName": "my-app",
		},
		CLIOverrides: map[string]any{
			"cliFlag": "cliValue",
		},
	}

	mergedVals := map[string]any{
		"appName": "my-app",
		"cliFlag": "cliValue",
		"helmDefaults": map[string]any{
			"atomic":  true,
			"wait":    true,
			"timeout": 300,
		},
	}

	tmplData := NewEnvironmentTemplateData(env, "ns", mergedVals)
	require.NotNil(t, tmplData)

	assert.Equal(t, mergedVals, tmplData.Values, ".Values should contain full merged values")

	_, hasDefaults := tmplData.Environment.Values["helmDefaults"]
	assert.False(t, hasDefaults, ".Environment.Values should NOT contain Defaults (helmDefaults)")

	assert.Equal(t, "my-app", tmplData.Environment.Values["appName"],
		".Environment.Values should contain env Values")
	assert.Equal(t, "cliValue", tmplData.Environment.Values["cliFlag"],
		".Environment.Values should contain CLI overrides")
}
