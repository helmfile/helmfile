package app

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateHelmDiffVersion(t *testing.T) {
	tests := []struct {
		name            string
		pluginDir       string
		requiredVersion string
		Result          error
	}{
		{
			name:            "match helm diff version",
			pluginDir:       "../../test/plugins/helm-diff/3.5.0",
			requiredVersion: "3.5.0",
			Result:          nil,
		},
		{
			name:            "helm diff version is low",
			pluginDir:       "../../test/plugins/helm-diff/3.5.0",
			requiredVersion: "3.6.0",
			Result:          errors.New("the minimum version that depends on helm-diff is 3.6.0, the current version is 3.5.0"),
		},
		{
			name:            "helm diff version is low2",
			pluginDir:       "../../test/plugins/helm-diff/2.11.0",
			requiredVersion: "3.6.0",
			Result:          errors.New("the minimum version that depends on helm-diff is 3.6.0, the current version is 2.11.0+5"),
		},
		{
			name:            "helm diff not install",
			pluginDir:       "../../test/plugins/helm-diff/",
			requiredVersion: "3.6.0",
			Result:          errors.New("plugin diff not installed"),
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("HELM_PLUGINS", tt.pluginDir)
			run := Run{}
			err := run.ValidateHelmDiffVersion(tt.requiredVersion)
			if !assert.Equal(t, err, tt.Result) {
				t.Errorf("unexpected error: %v, \nresult: %v", err, tt.Result)
			}
		})
	}
}
