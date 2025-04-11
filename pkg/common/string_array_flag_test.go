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
			name:         "nil default",
			defaultValue: nil,
			expected:     []string{},
		},
		{
			name:         "empty default",
			defaultValue: []string{},
			expected:     []string{},
		},
		{
			name:         "non-empty default",
			defaultValue: []string{"value1", "value2"},
			expected:     []string{"value1", "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := NewStringArrayFlag(tt.defaultValue)

			// Check initial state
			assert.Equal(t, tt.expected, flag.Values(), "Values should match default")
			assert.False(t, flag.WasExplicitlySet(), "New flag should not be marked as explicitly set")
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
			name:         "nil default, set values",
			defaultValue: nil,
			setValue:     []string{"new1", "new2"},
			expected:     []string{"new1", "new2"},
		},
		{
			name:         "non-empty default, set empty",
			defaultValue: []string{"default1", "default2"},
			setValue:     []string{},
			expected:     []string{},
		},
		{
			name:         "non-empty default, set new values",
			defaultValue: []string{"default1", "default2"},
			setValue:     []string{"new1", "new2"},
			expected:     []string{"new1", "new2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := NewStringArrayFlag(tt.defaultValue)

			// Set the values
			flag.Set(tt.setValue)

			// Check state after setting
			assert.Equal(t, tt.expected, flag.Values(), "Values should match set values")
			assert.True(t, flag.WasExplicitlySet(), "Flag should be marked as explicitly set")
		})
	}
}

func TestStringArrayFlag_Add(t *testing.T) {
	tests := []struct {
		name         string
		defaultValue []string
		addValue     string
		expected     []string
	}{
		{
			name:         "nil default, add value",
			defaultValue: nil,
			addValue:     "new",
			expected:     []string{"new"},
		},
		{
			name:         "empty default, add value",
			defaultValue: []string{},
			addValue:     "new",
			expected:     []string{"new"},
		},
		{
			name:         "non-empty default, add value",
			defaultValue: []string{"default1", "default2"},
			addValue:     "new",
			expected:     []string{"default1", "default2", "new"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := NewStringArrayFlag(tt.defaultValue)

			// Add the value
			flag.Add(tt.addValue)

			// Check state after adding
			assert.Equal(t, tt.expected, flag.Values(), "Values should include added value")
			assert.True(t, flag.WasExplicitlySet(), "Flag should be marked as explicitly set")
		})
	}
}

func TestStringArrayFlag_MultipleOperations(t *testing.T) {
	flag := NewStringArrayFlag([]string{"initial"})

	// Initial state
	assert.Equal(t, []string{"initial"}, flag.Values())
	assert.False(t, flag.WasExplicitlySet())

	// Add a value
	flag.Add("added")
	assert.Equal(t, []string{"initial", "added"}, flag.Values())
	assert.True(t, flag.WasExplicitlySet())

	// Set completely new values
	flag.Set([]string{"new1", "new2"})
	assert.Equal(t, []string{"new1", "new2"}, flag.Values())
	assert.True(t, flag.WasExplicitlySet(), "Flag should remain explicitly set")

	// Add another value after Set
	flag.Add("added2")
	assert.Equal(t, []string{"new1", "new2", "added2"}, flag.Values())
	assert.True(t, flag.WasExplicitlySet())
}

func TestStringArrayFlag_Implementation(t *testing.T) {
	// Test that stringArrayFlag properly implements StringArrayFlag interface
	var _ StringArrayFlag = &stringArrayFlag{}
}

func TestStringArrayFlag_DefensiveCopy(t *testing.T) {
	// Test that modifying the original slice doesn't affect the flag
	original := []string{"value1", "value2"}
	flag := NewStringArrayFlag(original)

	// Verify initial state
	assert.Equal(t, []string{"value1", "value2"}, flag.Values())

	// Modify the original slice - should NOT affect the flag's internal state
	// because NewStringArrayFlag creates a defensive copy
	original[0] = "modified"
	original = append(original, "added")

	// Flag values should remain unchanged
	assert.Equal(t, []string{"value1", "value2"}, flag.Values())

	// Test that modifying the returned slice doesn't affect the flag
	values := flag.Values()
	values[0] = "modified"
	values = append(values, "added")

	// Flag values should remain unchanged because Values() returns a copy
	assert.Equal(t, []string{"value1", "value2"}, flag.Values())
}

func TestStringArrayFlag_SetDefensiveCopy(t *testing.T) {
	// Test that Set doesn't create a defensive copy (current implementation)
	flag := NewStringArrayFlag([]string{})

	// Create a slice to set
	setValues := []string{"value1", "value2"}
	flag.Set(setValues)

	// Verify state after setting
	assert.Equal(t, []string{"value1", "value2"}, flag.Values())

	// Modify the original slice - this WILL affect the flag's internal state
	// because Set doesn't create a defensive copy in the current implementation
	setValues[0] = "modified"

	// Flag values will reflect the modification
	assert.Equal(t, []string{"modified", "value2"}, flag.Values())
}
