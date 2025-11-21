package app

import (
	goContext "context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/state"
)

// TestGetHelmWithEmptyDefaultHelmBinary tests that getHelm properly defaults to "helm"
// when st.DefaultHelmBinary is empty. This addresses the issue where base files with
// environment secrets would fail with "exec: no command" error.
//
// Background: When a base file has environment secrets but doesn't specify helmBinary,
// the state.DefaultHelmBinary would be empty, causing helmexec.New to be called with
// an empty string, which results in "error determining helm version: exec: no command".
//
// The fix in app.getHelm() ensures that when st.DefaultHelmBinary is empty, it defaults
// to state.DefaultHelmBinary ("helm").
func TestGetHelmWithEmptyDefaultHelmBinary(t *testing.T) {
	// Test that app.getHelm() handles empty DefaultHelmBinary correctly by applying a default
	st := &state.HelmState{
		ReleaseSetSpec: state.ReleaseSetSpec{
			DefaultHelmBinary: "", // Empty, as would be the case for base files
		},
	}

	logger := newAppTestLogger()
	app := &App{
		OverrideHelmBinary:              "",
		OverrideKubeContext:             "",
		DisableKubeVersionAutoDetection: true,
		Logger:                          logger,
		Env:                             "default",
		ctx:                             goContext.Background(),
	}

	// This should NOT fail because app.getHelm() defaults empty DefaultHelmBinary to "helm"
	helm, err := app.getHelm(st)

	// Verify that no error occurred - the fix in app.getHelm() prevents the "exec: no command" error
	require.NoError(t, err, "getHelm should not fail when DefaultHelmBinary is empty (fix should apply default)")

	// Verify that a valid helm execer was returned
	require.NotNil(t, helm, "getHelm should return a valid helm execer")

	// Verify that the helm version is accessible (confirms the helm binary is valid)
	version := helm.GetVersion()
	require.NotNil(t, version, "helm version should be accessible")
}
