// cmd/diff_test.go
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/flags"
	"github.com/helmfile/helmfile/pkg/testcmd"
)

func TestNewDiffCmd(t *testing.T) {
	// Test the actual command properties
	globalCfg := config.NewGlobalImpl(&config.GlobalOptions{HelmBinary: "helm"})
	cmd := NewDiffCmd(globalCfg)
	assert.Equal(t, "diff", cmd.Use)

	// Use the test helper for testing flags
	helper := testcmd.TestDiffCmd()
	assert.Equal(t, helper.Cmd.Use, cmd.Use)

	// Get the names of registered flags
	registeredFlags := helper.Registry.GetRegisteredFlagNames()

	// Verify flags and values
	assert.Contains(t, registeredFlags, "include-crds")

	includeCRDs, exists := flags.GetFlagValue[bool](helper.Registry, "include-crds")
	assert.True(t, exists)
	assert.False(t, includeCRDs)

	// Test other flags if needed
	// For example, testing a string flag:
	// outputFormat, exists := flags.GetFlagValue[string](helper.Registry, "output")
	// assert.True(t, exists)
	// assert.Equal(t, "", outputFormat)  // Default should be empty string

	// Or testing a string slice flag:
	// suppress, exists := flags.GetFlagValue[[]string](helper.Registry, "suppress")
	// assert.True(t, exists)
	// assert.Empty(t, suppress)  // Default should be empty slice
}
