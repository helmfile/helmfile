package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyOptions_HandleFlag(t *testing.T) {
	options := NewApplyOptions()

	// Test handling include-crds flag
	includeCRDs := true
	handled := options.HandleFlag("include-crds", &includeCRDs, true)
	assert.True(t, handled, "include-crds flag should be handled")
	assert.True(t, options.IncludeCRDsFlag.WasExplicitlySet())
	assert.True(t, options.IncludeCRDsFlag.Value())

	// Test that flag is handled even when not changed
	handled = options.HandleFlag("include-crds", &includeCRDs, false)
	assert.True(t, handled, "include-crds flag should be handled even when not changed")

	// Test handling skip-crds flag
	skipCRDs := true
	handled = options.HandleFlag("skip-crds", &skipCRDs, true)
	assert.True(t, options.SkipCRDsFlag.WasExplicitlySet())
	assert.True(t, handled, "skip-crds flag should be handled")
	assert.True(t, options.SkipCRDsFlag.WasExplicitlySet())
	assert.True(t, options.SkipCRDsFlag.Value())
}

func TestApplyOptions_HandleFlag_UnknownFlag(t *testing.T) {
	options := NewApplyOptions()

	// Test handling a non-existent flag
	skipCRDs := true
	handled := options.HandleFlag("non-existent-flag", &skipCRDs, true)
	assert.False(t, handled, "non-existent flag should not be handled")
}

func TestApplyOptions_HandleFlag_NotBool(t *testing.T) {
	options := NewApplyOptions()

	// Test handling include-crds flag
	includeCRDs := true
	handled := options.HandleFlag("include-crds", &includeCRDs, true)
	assert.True(t, handled, "include-crds flag should be handled")
	assert.True(t, options.IncludeCRDsFlag.Value())

	// Test with incorrect value type
	stringValue := "not-a-bool"
	handled = options.HandleFlag("include-crds", &stringValue, true)
	assert.True(t, handled, "include-crds flag should be handled even with incorrect type")

	// Value should not change when type is incorrect
	assert.True(t, options.IncludeCRDsFlag.Value())
}
