package app

import (
	"fmt"
	"os"
	"testing"

	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/stretchr/testify/require"
)

// TestIsExplicitSelectorInheritanceEnabled tests the isExplicitSelectorInheritanceEnabled function
func TestIsExplicitSelectorInheritanceEnabled(t *testing.T) {
	//env var ExperimentalEnvVar is not set
	require.Empty(t, os.Getenv(envvar.Experimental))
	require.False(t, isExplicitSelectorInheritanceEnabled())

	//check for env var ExperimentalEnvVar set to true
	os.Setenv(envvar.Experimental, "true")
	require.True(t, isExplicitSelectorInheritanceEnabled())

	//check for env var ExperimentalEnvVar set to anything
	os.Setenv(envvar.Experimental, "anything")
	require.False(t, isExplicitSelectorInheritanceEnabled())

	//check for env var ExperimentalEnvVar set to ExperimentalSelectorExplicit
	os.Setenv(envvar.Experimental, ExperimentalSelectorExplicit)
	require.True(t, isExplicitSelectorInheritanceEnabled())

	// check for env var ExperimentalEnvVar set to a string that contains ExperimentalSelectorExplicit and ExperimentalEnvVar set to true
	os.Setenv(envvar.Experimental, fmt.Sprintf("%s-%s-%s", "a", ExperimentalSelectorExplicit, "b"))
	require.True(t, isExplicitSelectorInheritanceEnabled())

	// reset env var
	defer os.Unsetenv(envvar.Experimental)
}

// TestExperimentalModeEnabled tests the experimentalModeEnabled function
func TestExperimentalModeEnabled(t *testing.T) {
	//env var ExperimentalEnvVar is not set
	require.Empty(t, os.Getenv(envvar.Experimental))
	require.False(t, experimentalModeEnabled())

	//check for env var ExperimentalEnvVar set to anything
	os.Setenv(envvar.Experimental, "anything")
	require.False(t, experimentalModeEnabled())

	//check for env var ExperimentalEnvVar set to true
	os.Setenv(envvar.Experimental, "true")
	require.True(t, experimentalModeEnabled())

	// reset env var
	defer os.Unsetenv(envvar.Experimental)
}
