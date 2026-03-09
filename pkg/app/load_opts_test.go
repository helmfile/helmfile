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

// TestLoadOptsDeepCopyPreservesOverrideValuesAreCLI verifies that DeepCopy
// preserves the OverrideValuesAreCLI flag which is tagged yaml:"-".
func TestLoadOptsDeepCopyPreservesOverrideValuesAreCLI(t *testing.T) {
	lOld := LoadOpts{
		Selectors:  []string{"test"},
		CalleePath: "test",
	}
	lOld.Environment.OverrideValuesAreCLI = true

	lNew := lOld.DeepCopy()

	require.True(t, lNew.Environment.OverrideValuesAreCLI, "DeepCopy should preserve OverrideValuesAreCLI flag")
}
