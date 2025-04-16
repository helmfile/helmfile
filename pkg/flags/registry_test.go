package flags

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestNewGenericFlagRegistrar(t *testing.T) {
	registry := NewMockFlagRegistry()
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.values)
	assert.Len(t, registry.values, 0)
}

func TestGenericFlagRegistry_RegisterBoolFlag(t *testing.T) {
	registry := NewMockFlagRegistry()
	cmd := &cobra.Command{Use: "test"}

	var testFlag bool
	registry.RegisterBoolFlag(cmd, "test-flag", &testFlag, false, "Test flag")

	// Verify the flag was registered
	flag := cmd.Flags().Lookup("test-flag")
	assert.NotNil(t, flag)
	assert.Equal(t, "test-flag", flag.Name)
	assert.Equal(t, "false", flag.DefValue)
	assert.Equal(t, "Test flag", flag.Usage)

	// Verify the value was stored in the registry
	value, exists := registry.values["test-flag"]
	assert.True(t, exists)
	assert.Equal(t, &testFlag, value)

	// Test the generic GetFlagValue function
	flagValue, exists := GetFlagValue[bool](registry, "test-flag")
	assert.True(t, exists)
	assert.Equal(t, false, flagValue)
}

func TestGenericFlagRegistry_TransferFlags_NoChanges(t *testing.T) {
	registry := NewMockFlagRegistry()
	cmd := &cobra.Command{Use: "test"}

	var testFlag bool
	registry.RegisterBoolFlag(cmd, "test-flag", &testFlag, false, "Test flag")

	// Create a mock handler
	handler := NewMockFlagHandler()

	// Transfer flags (none changed)
	registry.TransferFlags(cmd, handler)

	// Verify the handler was called with the right parameters
	assert.Equal(t, &testFlag, handler.handledFlags["test-flag"])
	assert.False(t, handler.changedFlags["test-flag"])
}

func TestGenericFlagRegistry_TransferFlags_WithChanges(t *testing.T) {
	registry := NewMockFlagRegistry()
	cmd := &cobra.Command{Use: "test"}

	var testFlag bool
	registry.RegisterBoolFlag(cmd, "test-flag", &testFlag, false, "Test flag")

	// Simulate flag being set on command line
	err := cmd.Flags().Set("test-flag", "true")
	assert.NoError(t, err)
	testFlag = true // Value would be updated by cobra

	// Create a mock handler
	handler := NewMockFlagHandler()

	// Transfer flags (with changes)
	registry.TransferFlags(cmd, handler)

	// Verify the handler was called with the right parameters
	assert.Equal(t, &testFlag, handler.handledFlags["test-flag"])
	assert.True(t, handler.changedFlags["test-flag"])
	assert.True(t, *handler.handledFlags["test-flag"].(*bool))
}

func TestGenericFlagRegistry_TransferFlags_NonHandler(t *testing.T) {
	registry := NewMockFlagRegistry()
	cmd := &cobra.Command{Use: "test"}

	var testFlag bool
	registry.RegisterBoolFlag(cmd, "test-flag", &testFlag, false, "Test flag")

	// Use a non-handler type
	nonHandler := struct{}{}

	// This should not panic
	registry.TransferFlags(cmd, nonHandler)
}

func TestGenericFlagRegistry_MultipleFlags(t *testing.T) {
	registry := NewMockFlagRegistry()
	cmd := &cobra.Command{Use: "test"}

	var boolFlag bool
	var boolFlag2 bool

	registry.RegisterBoolFlag(cmd, "bool-flag", &boolFlag, false, "Boolean flag")
	registry.RegisterBoolFlag(cmd, "bool-flag2", &boolFlag2, true, "Another boolean flag")

	// Set one flag
	err := cmd.Flags().Set("bool-flag", "true")
	assert.NoError(t, err)
	boolFlag = true // Value would be updated by cobra

	// Create a mock handler
	handler := NewMockFlagHandler()

	// Transfer flags
	registry.TransferFlags(cmd, handler)

	// Verify both flags were handled correctly
	assert.Equal(t, &boolFlag, handler.handledFlags["bool-flag"])
	assert.True(t, handler.changedFlags["bool-flag"])
	assert.True(t, *handler.handledFlags["bool-flag"].(*bool))

	assert.Equal(t, &boolFlag2, handler.handledFlags["bool-flag2"])
	assert.False(t, handler.changedFlags["bool-flag2"])
	assert.True(t, *handler.handledFlags["bool-flag2"].(*bool)) // Default is true
}

func TestGenericFlagRegistry_GetRegisteredFlagNames(t *testing.T) {
	registry := NewMockFlagRegistry()

	// Register some test flags
	boolValue := false
	registry.values["bool-flag"] = &boolValue

	stringValue := "test"
	registry.values["string-flag"] = &stringValue

	stringSliceValue := []string{"one", "two", "three"}
	registry.values["string-slice-flag"] = &stringSliceValue

	// Test GetRegisteredFlagNames
	names := registry.GetRegisteredFlagNames()
	assert.Len(t, names, 3)
	assert.Contains(t, names, "bool-flag")
	assert.Contains(t, names, "string-flag")
	assert.Contains(t, names, "string-slice-flag")
}

func TestGenericFlagRegistry_IsFlagRegistered(t *testing.T) {
	registry := NewGenericFlagRegistry()

	// Initially, no flags are registered
	assert.False(t, registry.IsFlagRegistered("test-flag"))

	// Register a flag
	boolValue := false
	registry.values["test-flag"] = &boolValue

	// Now the flag should be registered
	assert.True(t, registry.IsFlagRegistered("test-flag"))

	// Check a non-existent flag
	assert.False(t, registry.IsFlagRegistered("non-existent"))

	// Register another flag
	stringValue := "test"
	registry.values["string-flag"] = &stringValue

	// Both flags should be registered
	assert.True(t, registry.IsFlagRegistered("test-flag"))
	assert.True(t, registry.IsFlagRegistered("string-flag"))

	// Case sensitivity check
	assert.False(t, registry.IsFlagRegistered("TEST-FLAG"))
}
