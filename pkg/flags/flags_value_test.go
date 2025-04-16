package flags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFlagValue(t *testing.T) {
	registry := NewMockFlagRegistry()

	// Register some test flags
	boolValue := false
	registry.values["bool-flag"] = &boolValue

	stringValue := "test"
	registry.values["string-flag"] = &stringValue

	stringSliceValue := []string{"one", "two", "three"}
	registry.values["string-slice-flag"] = &stringSliceValue

	intValue := 42
	registry.values["int-flag"] = &intValue

	// Test getting boolean flag
	gotBool, exists := GetFlagValue[bool](registry, "bool-flag")
	assert.True(t, exists)
	assert.Equal(t, false, gotBool)

	// Test getting string flag
	gotString, exists := GetFlagValue[string](registry, "string-flag")
	assert.True(t, exists)
	assert.Equal(t, "test", gotString)

	// Test getting string slice flag
	gotStringSlice, exists := GetFlagValue[[]string](registry, "string-slice-flag")
	assert.True(t, exists)
	assert.Equal(t, []string{"one", "two", "three"}, gotStringSlice)

	// Test getting int flag
	gotInt, exists := GetFlagValue[int](registry, "int-flag")
	assert.True(t, exists)
	assert.Equal(t, 42, gotInt)

	// Test getting non-existent flag
	_, exists = GetFlagValue[bool](registry, "non-existent")
	assert.False(t, exists)

	// Test getting flag with wrong type
	_, exists = GetFlagValue[string](registry, "bool-flag")
	assert.False(t, exists)
}

func TestGetBoolFlagValue(t *testing.T) {
	registry := NewMockFlagRegistry()

	// Register a boolean flag
	boolValue := false
	registry.values["bool-flag"] = &boolValue

	// Test getting the flag value
	value, exists := GetBoolFlagValue(registry, "bool-flag")
	assert.True(t, exists)
	assert.False(t, value)

	// Change the value
	*registry.values["bool-flag"].(*bool) = true

	// Test getting the updated value
	value, exists = GetBoolFlagValue(registry, "bool-flag")
	assert.True(t, exists)
	assert.True(t, value)

	// Test getting a non-existent flag
	value, exists = GetBoolFlagValue(registry, "non-existent")
	assert.False(t, exists)
	assert.False(t, value) // Default value for bool

	// Test getting a flag with wrong type
	stringValue := "test"
	registry.values["string-flag"] = &stringValue
	value, exists = GetBoolFlagValue(registry, "string-flag")
	assert.False(t, exists)
	assert.False(t, value) // Default value for bool
}

func TestGetStringFlagValue(t *testing.T) {
	registry := NewMockFlagRegistry()

	// Register a string flag
	stringValue := "test"
	registry.values["string-flag"] = &stringValue

	// Test getting the flag value
	value, exists := GetStringFlagValue(registry, "string-flag")
	assert.True(t, exists)
	assert.Equal(t, "test", value)

	// Change the value
	*registry.values["string-flag"].(*string) = "updated"

	// Test getting the updated value
	value, exists = GetStringFlagValue(registry, "string-flag")
	assert.True(t, exists)
	assert.Equal(t, "updated", value)

	// Test getting a non-existent flag
	value, exists = GetStringFlagValue(registry, "non-existent")
	assert.False(t, exists)
	assert.Equal(t, "", value) // Default value for string

	// Test getting a flag with wrong type
	boolValue := true
	registry.values["bool-flag"] = &boolValue
	value, exists = GetStringFlagValue(registry, "bool-flag")
	assert.False(t, exists)
	assert.Equal(t, "", value) // Default value for string
}

func TestGetStringSliceFlagValue(t *testing.T) {
	registry := NewMockFlagRegistry()

	// Register a string slice flag
	sliceValue := []string{"one", "two", "three"}
	registry.values["slice-flag"] = &sliceValue

	// Test getting the flag value
	value, exists := GetStringSliceFlagValue(registry, "slice-flag")
	assert.True(t, exists)
	assert.Equal(t, []string{"one", "two", "three"}, value)

	// Change the value
	*registry.values["slice-flag"].(*[]string) = []string{"updated"}

	// Test getting the updated value
	value, exists = GetStringSliceFlagValue(registry, "slice-flag")
	assert.True(t, exists)
	assert.Equal(t, []string{"updated"}, value)

	// Test getting a non-existent flag
	value, exists = GetStringSliceFlagValue(registry, "non-existent")
	assert.False(t, exists)
	assert.Nil(t, value) // Default value for slice

	// Test getting a flag with wrong type
	boolValue := true
	registry.values["bool-flag"] = &boolValue
	value, exists = GetStringSliceFlagValue(registry, "bool-flag")
	assert.False(t, exists)
	assert.Nil(t, value) // Default value for slice
}

func TestGetIntFlagValue(t *testing.T) {
	registry := NewMockFlagRegistry()

	// Register an int flag
	intValue := 42
	registry.values["int-flag"] = &intValue

	// Test getting the flag value
	value, exists := GetIntFlagValue(registry, "int-flag")
	assert.True(t, exists)
	assert.Equal(t, 42, value)

	// Change the value
	*registry.values["int-flag"].(*int) = 100

	// Test getting the updated value
	value, exists = GetIntFlagValue(registry, "int-flag")
	assert.True(t, exists)
	assert.Equal(t, 100, value)

	// Test getting a non-existent flag
	value, exists = GetIntFlagValue(registry, "non-existent")
	assert.False(t, exists)
	assert.Equal(t, 0, value) // Default value for int

	// Test getting a flag with wrong type
	boolValue := true
	registry.values["bool-flag"] = &boolValue
	value, exists = GetIntFlagValue(registry, "bool-flag")
	assert.False(t, exists)
	assert.Equal(t, 0, value) // Default value for int
}

func TestGetInt64FlagValue(t *testing.T) {
	registry := NewMockFlagRegistry()

	// Register an int64 flag
	int64Value := int64(42)
	registry.values["int64-flag"] = &int64Value

	// Test getting the flag value
	value, exists := GetInt64FlagValue(registry, "int64-flag")
	assert.True(t, exists)
	assert.Equal(t, int64(42), value)

	// Test getting a non-existent flag
	value, exists = GetInt64FlagValue(registry, "non-existent")
	assert.False(t, exists)
	assert.Equal(t, int64(0), value) // Default value for int64
}

func TestGetFloat64FlagValue(t *testing.T) {
	registry := NewMockFlagRegistry()

	// Register a float64 flag
	float64Value := 3.14
	registry.values["float64-flag"] = &float64Value

	// Test getting the flag value
	value, exists := GetFloat64FlagValue(registry, "float64-flag")
	assert.True(t, exists)
	assert.Equal(t, 3.14, value)

	// Test getting a non-existent flag
	value, exists = GetFloat64FlagValue(registry, "non-existent")
	assert.False(t, exists)
	assert.Equal(t, 0.0, value) // Default value for float64
}
