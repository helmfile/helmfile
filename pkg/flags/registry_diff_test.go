package flags

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestDiffFlagRegisty(t *testing.T) {
	registry := NewDiffFlagRegistry()

	// Create a test command to register flags
	cmd := &cobra.Command{Use: "test"}
	registry.RegisterFlags(cmd)

	// Get the names of registered flags
	registeredFlags := registry.GetRegisteredFlagNames()

	// Verify that include-crds and skip-crds are registered
	assert.Contains(t, registeredFlags, "include-crds")

	// Get and verify the default values using the generic function
	includeCRDs, exists := GetFlagValue[bool](registry, "include-crds")
	assert.True(t, exists)
	assert.False(t, includeCRDs)
}
