package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/flags"
)

func TestSyncOptionsFactory_CreateOptions(t *testing.T) {
	factory := NewSyncOptionsFactory()

	// Test that CreateOptions returns a properly initialized SyncOptions
	options := factory.CreateOptions()

	// Type assertion
	syncOptions, ok := options.(*config.SyncOptions)
	assert.True(t, ok, "Expected *config.SyncOptions, got %T", options)

	// Verify default values
	assert.False(t, syncOptions.Validate)
	assert.False(t, syncOptions.SkipNeeds)

	// Verify BoolFlag initialization
	assert.False(t, syncOptions.IncludeCRDsFlag.Value())
	assert.False(t, syncOptions.SkipCRDsFlag.Value())
}

func TestSyncOptionsFactory_GetFlagRegistrar(t *testing.T) {
	factory := NewSyncOptionsFactory()

	// Test that GetFlagRegistrar returns a SyncFlagRegistrar
	registry := factory.GetFlagRegistry()

	// Type assertion
	_, ok := registry.(*flags.SyncFlagRegistry)
	assert.True(t, ok, "Expected *flags.SyncFlagRegistrar, got %T", registry)
}
