package flags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockFlagHandler_HandleFlag_ReturnValue(t *testing.T) {
	// Create a mock handler
	mockHandler := NewMockFlagHandler()

	// Test that it returns true for any flag
	result := mockHandler.HandleFlag("test-flag", true, false)
	assert.True(t, result, "MockHandler should return true for any flag")

	// Test with multiple flags
	result = mockHandler.HandleFlag("another-flag", "value", true)
	assert.True(t, result, "MockHandler should return true for any flag")

	// Verify the flags were stored correctly
	assert.Equal(t, true, mockHandler.handledFlags["test-flag"])
	assert.Equal(t, "value", mockHandler.handledFlags["another-flag"])
	assert.Equal(t, false, mockHandler.changedFlags["test-flag"])
	assert.Equal(t, true, mockHandler.changedFlags["another-flag"])
}

// CustomFlagHandler implements FlagHandler for testing specific return values
type CustomFlagHandler struct{}

func (h *CustomFlagHandler) HandleFlag(name string, value interface{}, changed bool) bool {
	// Only handle specific flags
	switch name {
	case "known-flag":
		return true
	default:
		return false
	}
}

func TestCustomFlagHandler_HandleFlag_ReturnValue(t *testing.T) {
	handler := &CustomFlagHandler{}

	// Test with a recognized flag
	result := handler.HandleFlag("known-flag", true, false)
	assert.True(t, result, "Should return true for recognized flag")

	// Test with an unrecognized flag
	result = handler.HandleFlag("unknown-flag", true, false)
	assert.False(t, result, "Should return false for unrecognized flag")
}
