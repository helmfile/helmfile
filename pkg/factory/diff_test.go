package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/flags"
)

func TestDiffOptionsFactory_CreateOptions(t *testing.T) {
	factory := NewDiffOptionsFactory()

	// Test that CreateOptions returns a properly initialized DiffOptions
	options := factory.CreateOptions()

	// Type assertion
	diffOptions, ok := options.(*config.DiffOptions)
	assert.True(t, ok, "Expected *config.DiffOptions, got %T", options)

	// Verify default values
	assert.False(t, diffOptions.DetailedExitcode)
	assert.False(t, diffOptions.StripTrailingCR)
	assert.False(t, diffOptions.IncludeTests)
	assert.False(t, diffOptions.SuppressSecrets)
	assert.False(t, diffOptions.ShowSecrets)
	assert.False(t, diffOptions.NoHooks)

	// Verify BoolFlag initialization
	assert.False(t, diffOptions.IncludeCRDsFlag.Value())
	assert.False(t, diffOptions.SkipCRDsFlag.Value())
}

func TestDiffOptionsFactory_GetFlagRegistrar(t *testing.T) {
	factory := NewDiffOptionsFactory()

	// Test that GetFlagRegistrar returns a DiffFlagRegistrar
	registry := factory.GetFlagRegistry()

	// Type assertion
	_, ok := registry.(*flags.DiffFlagRegistry)
	assert.True(t, ok, "Expected *flags.DiffFlagRegistrar, got %T", registry)
}
