package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFactoryInterface ensures all factories implement the OptionsFactory interface
func TestFactoryInterface(t *testing.T) {
	// Test that each factory implements OptionsFactory
	var _ OptionsFactory = &DiffOptionsFactory{}
	var _ OptionsFactory = &ApplyOptionsFactory{}
	var _ OptionsFactory = &SyncOptionsFactory{}
	var _ OptionsFactory = &TemplateOptionsFactory{}
}

// TestFactoryCreation tests the creation of factories
func TestFactoryCreation(t *testing.T) {
	// Test that each factory can be created
	diffFactory := NewDiffOptionsFactory()
	assert.NotNil(t, diffFactory)

	applyFactory := NewApplyOptionsFactory()
	assert.NotNil(t, applyFactory)

	syncFactory := NewSyncOptionsFactory()
	assert.NotNil(t, syncFactory)

	templateFactory := NewTemplateOptionsFactory()
	assert.NotNil(t, templateFactory)
}
