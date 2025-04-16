package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/flags"
)

func TestApplyOptionsFactory_CreateOptions(t *testing.T) {
	factory := NewApplyOptionsFactory()

	// Test that CreateOptions returns a properly initialized ApplyOptions
	options := factory.CreateOptions()

	// Type assertion
	applyOptions, ok := options.(*config.ApplyOptions)
	assert.True(t, ok, "Expected *config.ApplyOptions, got %T", options)

	// Verify default values
	assert.False(t, applyOptions.DetailedExitcode)
	assert.False(t, applyOptions.StripTrailingCR)
	assert.False(t, applyOptions.IncludeTests)
	assert.False(t, applyOptions.SuppressSecrets)
	assert.False(t, applyOptions.ShowSecrets)
	assert.False(t, applyOptions.NoHooks)
	assert.False(t, applyOptions.SkipNeeds)

	// Verify BoolFlag initialization
	assert.False(t, applyOptions.IncludeCRDsFlag.Value())
	assert.False(t, applyOptions.SkipCRDsFlag.Value())
}

func TestApplyOptionsFactory_GetFlagRegistrar(t *testing.T) {
	factory := NewApplyOptionsFactory()

	// Test that GetFlagRegistrar returns an ApplyFlagRegistrar
	registry := factory.GetFlagRegistry()

	// Type assertion
	_, ok := registry.(*flags.ApplyFlagRegistry)
	assert.True(t, ok, "Expected *flags.ApplyFlagRegistrar, got %T", registry)
}
