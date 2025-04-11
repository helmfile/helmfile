package flags

import (
    "testing"

    "github.com/spf13/cobra"
    "github.com/stretchr/testify/assert"
)

// MockFlagHandler implements FlagHandler for testing
type MockFlagHandler struct {
    handledFlags map[string]interface{}
    changedFlags map[string]bool
}

func NewMockFlagHandler() *MockFlagHandler {
    return &MockFlagHandler{
        handledFlags: make(map[string]interface{}),
        changedFlags: make(map[string]bool),
    }
}

func (h *MockFlagHandler) HandleFlag(name string, value interface{}, changed bool) {
    h.handledFlags[name] = value
    h.changedFlags[name] = changed
}

func TestNewGenericFlagRegistrar(t *testing.T) {
    registrar := NewGenericFlagRegistrar()
    assert.NotNil(t, registrar)
    assert.NotNil(t, registrar.values)
    assert.Len(t, registrar.values, 0)
}

func TestGenericFlagRegistrar_RegisterBoolFlag(t *testing.T) {
    registrar := NewGenericFlagRegistrar()
    cmd := &cobra.Command{Use: "test"}

    var testFlag bool
    registrar.RegisterBoolFlag(cmd, "test-flag", &testFlag, false, "Test flag")

    // Verify the flag was registered
    flag := cmd.Flags().Lookup("test-flag")
    assert.NotNil(t, flag)
    assert.Equal(t, "test-flag", flag.Name)
    assert.Equal(t, "false", flag.DefValue)
    assert.Equal(t, "Test flag", flag.Usage)

    // Verify the value was stored in the registrar
    value, exists := registrar.values["test-flag"]
    assert.True(t, exists)
    assert.Equal(t, &testFlag, value)
}

func TestGenericFlagRegistrar_TransferFlags_NoChanges(t *testing.T) {
    registrar := NewGenericFlagRegistrar()
    cmd := &cobra.Command{Use: "test"}

    var testFlag bool
    registrar.RegisterBoolFlag(cmd, "test-flag", &testFlag, false, "Test flag")

    // Create a mock handler
    handler := NewMockFlagHandler()

    // Transfer flags (none changed)
    registrar.TransferFlags(cmd, handler)

    // Verify the handler was called with the right parameters
    assert.Equal(t, &testFlag, handler.handledFlags["test-flag"])
    assert.False(t, handler.changedFlags["test-flag"])
}

func TestGenericFlagRegistrar_TransferFlags_WithChanges(t *testing.T) {
    registrar := NewGenericFlagRegistrar()
    cmd := &cobra.Command{Use: "test"}

    var testFlag bool
    registrar.RegisterBoolFlag(cmd, "test-flag", &testFlag, false, "Test flag")

    // Simulate flag being set on command line
    err := cmd.Flags().Set("test-flag", "true")
    assert.NoError(t, err)
    testFlag = true // Value would be updated by cobra

    // Create a mock handler
    handler := NewMockFlagHandler()

    // Transfer flags (with changes)
    registrar.TransferFlags(cmd, handler)

    // Verify the handler was called with the right parameters
    assert.Equal(t, &testFlag, handler.handledFlags["test-flag"])
    assert.True(t, handler.changedFlags["test-flag"])
    assert.True(t, *handler.handledFlags["test-flag"].(*bool))
}

func TestGenericFlagRegistrar_TransferFlags_NonHandler(t *testing.T) {
    registrar := NewGenericFlagRegistrar()
    cmd := &cobra.Command{Use: "test"}

    var testFlag bool
    registrar.RegisterBoolFlag(cmd, "test-flag", &testFlag, false, "Test flag")

    // Use a non-handler type
    nonHandler := struct{}{}

    // This should not panic
    registrar.TransferFlags(cmd, nonHandler)
}

func TestGenericFlagRegistrar_MultipleFlags(t *testing.T) {
    registrar := NewGenericFlagRegistrar()
    cmd := &cobra.Command{Use: "test"}

    var boolFlag bool
    var boolFlag2 bool

    registrar.RegisterBoolFlag(cmd, "bool-flag", &boolFlag, false, "Boolean flag")
    registrar.RegisterBoolFlag(cmd, "bool-flag2", &boolFlag2, true, "Another boolean flag")

    // Set one flag
    err := cmd.Flags().Set("bool-flag", "true")
    assert.NoError(t, err)
    boolFlag = true // Value would be updated by cobra

    // Create a mock handler
    handler := NewMockFlagHandler()

    // Transfer flags
    registrar.TransferFlags(cmd, handler)

    // Verify both flags were handled correctly
    assert.Equal(t, &boolFlag, handler.handledFlags["bool-flag"])
    assert.True(t, handler.changedFlags["bool-flag"])
    assert.True(t, *handler.handledFlags["bool-flag"].(*bool))

    assert.Equal(t, &boolFlag2, handler.handledFlags["bool-flag2"])
    assert.False(t, handler.changedFlags["bool-flag2"])
    assert.True(t, *handler.handledFlags["bool-flag2"].(*bool)) // Default is true
}
