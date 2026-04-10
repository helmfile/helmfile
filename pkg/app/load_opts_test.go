package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLoadOptsDeepCopy tests the DeepCopy function for LoadOpts struct.
func TestLoadOptsDeepCopy(t *testing.T) {
	lOld := LoadOpts{
		Selectors:         []string{"test"},
		RetainValuesFiles: true,
		CalleePath:        "test",
		Reverse:           true,
		Filter:            true,
	}
	lNew := lOld.DeepCopy()

	// Check that the new struct is not the same as the old one.
	require.Equal(t, lOld, lNew, "DeepCopy should return a copy of the LoadOpts struct")
}

// TestLoadOptsDeepCopyPreservesOverrideCLISetValues verifies that DeepCopy
// preserves the OverrideCLISetValues field which is tagged yaml:"-".
func TestLoadOptsDeepCopyPreservesOverrideCLISetValues(t *testing.T) {
	lOld := LoadOpts{
		Selectors:  []string{"test"},
		CalleePath: "test",
	}
	lOld.Environment.OverrideCLISetValues = []any{map[string]any{"key": "value"}}

	lNew := lOld.DeepCopy()

	require.Equal(t, lOld.Environment.OverrideCLISetValues, lNew.Environment.OverrideCLISetValues, "DeepCopy should preserve OverrideCLISetValues field")
}
