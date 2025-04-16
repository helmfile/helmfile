package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStringFlag(t *testing.T) {
	tests := []struct {
		name         string
		defaultValue string
		expected     string
	}{
		{
			name:         "empty default",
			defaultValue: "",
			expected:     "",
		},
		{
			name:         "non-empty default",
			defaultValue: "default",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := NewStringFlag(tt.defaultValue)

			// Check initial state
			assert.Equal(t, tt.expected, flag.Value(), "Value should match default")
			assert.False(t, flag.WasExplicitlySet(), "New flag should not be marked as explicitly set")
		})
	}
}

func TestStringFlag_Set(t *testing.T) {
	tests := []struct {
		name         string
		defaultValue string
		setValue     string
		expected     string
	}{
		{
			name:         "empty default, set value",
			defaultValue: "",
			setValue:     "new value",
			expected:     "new value",
		},
		{
			name:         "non-empty default, set empty",
			defaultValue: "default",
			setValue:     "",
			expected:     "",
		},
		{
			name:         "non-empty default, set new value",
			defaultValue: "default",
			setValue:     "new value",
			expected:     "new value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := NewStringFlag(tt.defaultValue)

			// Set the value
			flag.Set(tt.setValue)

			// Check state after setting
			assert.Equal(t, tt.expected, flag.Value(), "Value should match set value")
			assert.True(t, flag.WasExplicitlySet(), "Flag should be marked as explicitly set")
		})
	}
}

func TestStringFlag_MultipleSet(t *testing.T) {
	flag := NewStringFlag("initial")

	// Initial state
	assert.Equal(t, "initial", flag.Value())
	assert.False(t, flag.WasExplicitlySet())

	// First set
	flag.Set("first")
	assert.Equal(t, "first", flag.Value())
	assert.True(t, flag.WasExplicitlySet())

	// Second set
	flag.Set("second")
	assert.Equal(t, "second", flag.Value())
	assert.True(t, flag.WasExplicitlySet(), "Flag should remain explicitly set")
}

func TestStringFlag_Implementation(t *testing.T) {
	// Test that stringFlag properly implements StringFlag interface
	var _ StringFlag = &stringFlag{}
}
