package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMapFlag(t *testing.T) {
	tests := []struct {
		name         string
		defaultValue map[string]string
		expected     map[string]string
	}{
		{
			name:         "default empty",
			defaultValue: map[string]string{},
			expected:     map[string]string{},
		},
		{
			name:         "default with values",
			defaultValue: map[string]string{"key1": "value1", "key2": "value2"},
			expected:     map[string]string{"key1": "value1", "key2": "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := NewMapFlag(tt.defaultValue)

			// Check initial state
			assert.Equal(t, tt.expected, flag.Value(), "Value should match default")
			assert.False(t, flag.WasExplicitlySet(), "New flag should not be marked as explicitly set")

			// Ensure the default value is copied, not referenced
			if len(tt.defaultValue) > 0 {
				original := make(map[string]string)
				for k, v := range tt.defaultValue {
					original[k] = v
				}

				// Modify the original
				for k := range tt.defaultValue {
					tt.defaultValue[k] = "modified"
					break
				}

				// Flag value should remain unchanged
				assert.Equal(t, original, flag.Value(), "Flag value should be a copy, not a reference")
			}
		})
	}
}

func TestMapFlag_ValueImmutability(t *testing.T) {
	// Test that modifying the returned value doesn't affect the internal state
	flag := NewMapFlag(map[string]string{"key1": "value1", "key2": "value2"})

	// Get the value and modify it
	value := flag.Value()
	value["key1"] = "modified"

	// Check that the flag's internal state is unchanged
	expected := map[string]string{"key1": "value1", "key2": "value2"}
	assert.Equal(t, expected, flag.Value(), "Modifying the returned value should not affect the flag's internal state")
}

func TestMapFlag_Set(t *testing.T) {
	tests := []struct {
		name         string
		defaultValue map[string]string
		setValue     map[string]string
		expected     map[string]string
	}{
		{
			name:         "default empty, set values",
			defaultValue: map[string]string{},
			setValue:     map[string]string{"key1": "value1", "key2": "value2"},
			expected:     map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name:         "default with values, set new values",
			defaultValue: map[string]string{"key1": "value1", "key2": "value2"},
			setValue:     map[string]string{"key3": "value3", "key4": "value4"},
			expected:     map[string]string{"key3": "value3", "key4": "value4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := NewMapFlag(tt.defaultValue)

			// Set the value
			flag.Set(tt.setValue)

			// Check state after setting
			assert.Equal(t, tt.expected, flag.Value(), "Value should match set value")
			assert.True(t, flag.WasExplicitlySet(), "Flag should be marked as explicitly set")

			// Ensure the set value is copied, not referenced
			if len(tt.setValue) > 0 {
				original := make(map[string]string)
				for k, v := range tt.setValue {
					original[k] = v
				}

				// Modify the original
				for k := range tt.setValue {
					tt.setValue[k] = "modified"
					break
				}

				// Flag value should remain unchanged
				assert.Equal(t, original, flag.Value(), "Flag value should be a copy, not a reference")
			}
		})
	}
}

func TestMapFlag_SetKey(t *testing.T) {
	flag := NewMapFlag(map[string]string{"existing": "value"})

	// Initial state
	assert.Equal(t, map[string]string{"existing": "value"}, flag.Value())
	assert.False(t, flag.WasExplicitlySet())

	// Set a new key
	flag.SetKey("new", "value")
	assert.Equal(t, map[string]string{"existing": "value", "new": "value"}, flag.Value())
	assert.True(t, flag.WasExplicitlySet())

	// Override an existing key
	flag.SetKey("existing", "updated")
	assert.Equal(t, map[string]string{"existing": "updated", "new": "value"}, flag.Value())
	assert.True(t, flag.WasExplicitlySet())
}

func TestMapFlag_MultipleSet(t *testing.T) {
	flag := NewMapFlag(map[string]string{"initial": "value"})

	// Initial state
	assert.Equal(t, map[string]string{"initial": "value"}, flag.Value())
	assert.False(t, flag.WasExplicitlySet())

	// First set
	flag.Set(map[string]string{"first": "set"})
	assert.Equal(t, map[string]string{"first": "set"}, flag.Value())
	assert.True(t, flag.WasExplicitlySet())

	// Second set
	flag.Set(map[string]string{"second": "set"})
	assert.Equal(t, map[string]string{"second": "set"}, flag.Value())
	assert.True(t, flag.WasExplicitlySet(), "Flag should remain explicitly set")
}

func TestMapFlag_Implementation(t *testing.T) {
	// Test that mapFlag properly implements MapFlag interface
	var _ MapFlag = &mapFlag{}
}
