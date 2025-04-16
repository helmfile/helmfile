// cmd/diff_test.go
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/flags"
	"github.com/helmfile/helmfile/pkg/testcmd"
)

func TestNewApplyCmd(t *testing.T) {
	// Test the actual command properties
	globalCfg := config.NewGlobalImpl(&config.GlobalOptions{HelmBinary: "helm"})
	cmd := NewApplyCmd(globalCfg)
	assert.Equal(t, "apply", cmd.Use)

	// Use the test helper for testing flags
	helper := testcmd.TestApplyCmd()
	assert.Equal(t, helper.Cmd.Use, cmd.Use)

	// Get the names of registered flags
	registeredFlags := helper.Registry.GetRegisteredFlagNames()

	// Verify flags and values
	assert.Contains(t, registeredFlags, "include-crds")
	assert.Contains(t, registeredFlags, "skip-crds")

	includeCRDs, exists := flags.GetFlagValue[bool](helper.Registry, "include-crds")
	assert.True(t, exists)
	assert.False(t, includeCRDs)

	skipCRDs, exists := flags.GetFlagValue[bool](helper.Registry, "skip-crds")
	assert.True(t, exists)
	assert.False(t, skipCRDs)
}
