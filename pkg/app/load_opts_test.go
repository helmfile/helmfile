package app

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/state"
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

// TestLoadOptsDeepCopyPreservesInheritedPureFields verifies DeepCopy preserves
// the yaml-tagged fields of Inherited (repositories etc.) via the yaml round-trip.
func TestLoadOptsDeepCopyPreservesInheritedPureFields(t *testing.T) {
	lOld := LoadOpts{CalleePath: "test"}
	lOld.Inherited = &state.InheritedConfig{
		Repositories: []state.RepositorySpec{{Name: "a", URL: "u"}},
	}

	lNew := lOld.DeepCopy()

	require.NotNil(t, lNew.Inherited)
	require.Equal(t, lOld.Inherited.Repositories, lNew.Inherited.Repositories,
		"DeepCopy should preserve Inherited.Repositories")
}

// TestLoadOptsDeepCopyPreservesInheritedEnv verifies DeepCopy preserves
// Inherited.Env, which is tagged yaml:"-" and therefore needs explicit handling.
func TestLoadOptsDeepCopyPreservesInheritedEnv(t *testing.T) {
	lOld := LoadOpts{CalleePath: "test"}
	env := environment.Environment{Name: "prod", Values: map[string]any{"k": "v"}}
	lOld.Inherited = &state.InheritedConfig{Env: &env}

	lNew := lOld.DeepCopy()

	require.NotNil(t, lNew.Inherited, "Inherited must survive round-trip even with only Env set")
	require.NotNil(t, lNew.Inherited.Env, "Env must survive DeepCopy (it is yaml:\"-\")")
	require.Equal(t, "prod", lNew.Inherited.Env.Name)
	require.Equal(t, "v", lNew.Inherited.Env.Values["k"])
}

// TestLoadOptsDeepCopyEnvIsNotShallow verifies the deep-copied Env is not
// aliased to the original.
func TestLoadOptsDeepCopyEnvIsNotShallow(t *testing.T) {
	lOld := LoadOpts{CalleePath: "test"}
	env := environment.Environment{Values: map[string]any{"k": "original"}}
	lOld.Inherited = &state.InheritedConfig{Env: &env}

	lNew := lOld.DeepCopy()
	lNew.Inherited.Env.Values["k"] = "mutated"

	require.Equal(t, "original", lOld.Inherited.Env.Values["k"],
		"mutating the copy's Env must not affect the original (aliasing bug)")
}

// TestLoadOptsDeepCopyPreservesParentRepoNames verifies the warning field
// survives DeepCopy.
func TestLoadOptsDeepCopyPreservesParentRepoNames(t *testing.T) {
	lOld := LoadOpts{CalleePath: "test", ParentRepoNames: []string{"a", "b"}}

	lNew := lOld.DeepCopy()

	require.Equal(t, lOld.ParentRepoNames, lNew.ParentRepoNames)
}
