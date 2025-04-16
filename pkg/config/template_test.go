package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplateOptions_HandleFlag(t *testing.T) {
	options := NewTemplateOptions()

	// Test handling include-crds flag
	includeCRDs := true
	handled := options.HandleFlag("include-crds", &includeCRDs, true)
	assert.True(t, handled, "include-crds flag should be handled")
	assert.True(t, options.IncludeCRDsFlag.Value())
}
