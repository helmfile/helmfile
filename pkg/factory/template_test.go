package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/flags"
)

func TestTemplateOptionsFactory_CreateOptions(t *testing.T) {
	factory := NewTemplateOptionsFactory()

	// Test that CreateOptions returns a properly initialized TemplateOptions
	options := factory.CreateOptions()

	// Type assertion
	templateOptions, ok := options.(*config.TemplateOptions)
	assert.True(t, ok, "Expected *config.TemplateOptions, got %T", options)

	// Verify default values
	assert.False(t, templateOptions.SkipNeeds)
	assert.False(t, templateOptions.SkipTests)
	assert.False(t, templateOptions.NoHooks)

	// Verify BoolFlag initialization
	assert.False(t, templateOptions.IncludeCRDsFlag.Value())
	assert.False(t, templateOptions.SkipCRDsFlag.Value())
}

func TestTemplateOptionsFactory_GetFlagRegistry(t *testing.T) {
	factory := NewTemplateOptionsFactory()

	// Test that GetFlagRegistrar returns a TemplateFlagRegistry
	registry := factory.GetFlagRegistry()

	// Type assertion
	_, ok := registry.(*flags.TemplateFlagRegistry)
	assert.True(t, ok, "Expected *flags.TemplateFlagRegistry, got %T", registry)
}
