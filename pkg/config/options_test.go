package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffOptions_Initialize(t *testing.T) {
	options := &DiffOptions{}
	options.Initialize()

	// Verify initialization
	assert.False(t, options.DetailedExitcode)
	assert.False(t, options.StripTrailingCR)
	assert.False(t, options.IncludeTests)
	assert.False(t, options.SuppressSecrets)
	assert.False(t, options.ShowSecrets)
	assert.False(t, options.NoHooks)
	assert.False(t, options.IncludeCRDsFlag.Value())
	assert.False(t, options.SkipCRDsFlag.Value())
}

func TestApplyOptions_Initialize(t *testing.T) {
	options := &ApplyOptions{}
	options.Initialize()

	// Verify initialization
	assert.False(t, options.DetailedExitcode)
	assert.False(t, options.StripTrailingCR)
	assert.False(t, options.IncludeTests)
	assert.False(t, options.SuppressSecrets)
	assert.False(t, options.ShowSecrets)
	assert.False(t, options.NoHooks)
	assert.False(t, options.SkipNeeds)
	assert.False(t, options.IncludeCRDsFlag.Value())
	assert.False(t, options.SkipCRDsFlag.Value())
}

func TestSyncOptions_Initialize(t *testing.T) {
	options := &SyncOptions{}
	options.Initialize()

	// Verify initialization
	assert.False(t, options.Validate)
	assert.False(t, options.SkipNeeds)
	assert.False(t, options.IncludeCRDsFlag.Value())
	assert.False(t, options.SkipCRDsFlag.Value())
}

func TestTemplateOptions_Initialize(t *testing.T) {
	options := &TemplateOptions{}
	options.Initialize()

	// Verify initialization
	assert.False(t, options.SkipNeeds)
	assert.False(t, options.SkipTests)
	assert.False(t, options.NoHooks)
	assert.False(t, options.IncludeCRDsFlag.Value())
	assert.False(t, options.SkipCRDsFlag.Value())
}
