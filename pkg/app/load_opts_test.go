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

// TestLoadOptsDeepCopyOverrideCLISetValuesIsNotShallow verifies that mutating a
// map nested inside OverrideCLISetValues on the copy does not affect the
// original.
func TestLoadOptsDeepCopyOverrideCLISetValuesIsNotShallow(t *testing.T) {
	lOld := LoadOpts{}
	lOld.Environment.OverrideCLISetValues = []any{map[string]any{"key": "original"}}

	lNew := lOld.DeepCopy()

	// Mutate the map inside the copy.
	lNew.Environment.OverrideCLISetValues[0].(map[string]any)["key"] = "mutated"

	// The original must be unaffected; this fails with a shallow copy.
	require.Equal(t, "original", lOld.Environment.OverrideCLISetValues[0].(map[string]any)["key"],
		"mutating the copy's OverrideCLISetValues map must not affect the original (aliasing bug)")
}
