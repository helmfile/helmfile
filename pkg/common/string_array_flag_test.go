package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStringArrayFlag(t *testing.T) {
	tests := []struct {
		name         string
		defaultValue []string
		expected     []string
	}{
		{
			name:         "default empty",
			defaultValue: []string{},
			expected:     []string{},
		},
		{
			name:         "default with values",
			defaultValue: []string{"one", "two"},
			expected:     []string{"one", "two"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := NewStringArrayFlag(tt.defaultValue)

			// Check initial state
			assert.Equal(t, tt.expected, flag.Value(), "Value should match default")
			assert.False(t, flag.WasExplicitlySet(), "New flag should not be marked as explicitly set")

			// Ensure the default value is copied, not referenced
			if len(tt.defaultValue) > 0 {
				original := make([]string, len(tt.defaultValue))
				copy(original, tt.defaultValue)

				// Modify the original
				tt.defaultValue[0] = "modified"

				// Flag value should remain unchanged
				assert.Equal(t, original, flag.Value(), "Flag value should be a copy, not a reference")
			}
		})
	}
}

func TestStringArrayFlag_Set(t *testing.T) {
	tests := []struct {
		name         string
		defaultValue []string
		setValue     []string
		expected     []string
	}{
		{
			name:         "default empty, set values",
			defaultValue: []string{},
			setValue:     []string{"one", "two"},
			expected:     []string{"one", "two"},
		},
		{
			name:         "default with values, set new values",
			defaultValue: []string{"one", "two"},
			setValue:     []string{"three", "four"},
			expected:     []string{"three", "four"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := NewStringArrayFlag(tt.defaultValue)

			// Set the value
			flag.Set(tt.setValue)

			// Check state after setting
			assert.Equal(t, tt.expected, flag.Value(), "Value should match set value")
			assert.True(t, flag.WasExplicitlySet(), "Flag should be marked as explicitly set")

			// Ensure the set value is copied, not referenced
			if len(tt.setValue) > 0 {
				original := make([]string, len(tt.setValue))
				copy(original, tt.setValue)

				// Modify the original
				tt.setValue[0] = "modified"

				// Flag value should remain unchanged
				assert.Equal(t, original, flag.Value(), "Flag value should be a copy, not a reference")
			}
		})
	}
}

func TestStringArrayFlag_ValueImmutability(t *testing.T) {
	// Test that modifying the returned value doesn't affect the internal state
	flag := NewStringArrayFlag([]string{"one", "two"})

	// Get the value and modify it
	value := flag.Value()
	value[0] = "modified"

	// Check that the flag's internal state is unchanged
	assert.Equal(t, []string{"one", "two"}, flag.Value(), "Modifying the returned value should not affect the flag's internal state")
}

func TestStringArrayFlag_Append(t *testing.T) {
	flag := NewStringArrayFlag([]string{"one"})

	// Initial state
	assert.Equal(t, []string{"one"}, flag.Value())
	assert.False(t, flag.WasExplicitlySet())

	// Append a value
	flag.Append("two")
	assert.Equal(t, []string{"one", "two"}, flag.Value())
	assert.True(t, flag.WasExplicitlySet())

	// Append another value
	flag.Append("three")
	assert.Equal(t, []string{"one", "two", "three"}, flag.Value())
	assert.True(t, flag.WasExplicitlySet())
}

func TestStringArrayFlag_MultipleSet(t *testing.T) {
	flag := NewStringArrayFlag([]string{"initial"})

	// Initial state
	assert.Equal(t, []string{"initial"}, flag.Value())
	assert.False(t, flag.WasExplicitlySet())

	// First set
	flag.Set([]string{"first", "set"})
	assert.Equal(t, []string{"first", "set"}, flag.Value())
	assert.True(t, flag.WasExplicitlySet())

	// Second set
	flag.Set([]string{"second", "set"})
	assert.Equal(t, []string{"second", "set"}, flag.Value())
	assert.True(t, flag.WasExplicitlySet(), "Flag should remain explicitly set")
}

func TestArrayStringFlag_Implementation(t *testing.T) {
	// Test that stringArrayFlag properly implements StringArrayFlag interface
	var _ StringArrayFlag = &stringArrayFlag{}
}
