package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncOptions_HandleFlag(t *testing.T) {
	options := NewSyncOptions()

	// Test handling include-crds flag
	includeCRDs := true
	handled := options.HandleFlag("include-crds", &includeCRDs, true)
	assert.True(t, handled, "include-crds flag should be handled")
	assert.True(t, options.IncludeCRDsFlag.Value())

	// Test handling skip-crds flag
	skipCRDs := true
	handled = options.HandleFlag("skip-crds", &skipCRDs, true)
	assert.True(t, handled, "skip-crds flag should be handled")
	assert.True(t, options.SkipCRDsFlag.Value())
}
