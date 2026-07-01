package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/yaml"
)

func TestSubHelmfileSpec_UnmarshalInherits(t *testing.T) {
	t.Run("map form parses inherits", func(t *testing.T) {
		var hf SubHelmfileSpec
		require.NoError(t, yaml.Unmarshal([]byte(`
path: myapp.yaml
inherits:
- repositories
- helmDefaults
`), &hf))
		assert.Equal(t, "myapp.yaml", hf.Path)
		assert.Equal(t, []string{"repositories", "helmDefaults"}, hf.Inherits)
	})

	t.Run("string shorthand leaves inherits nil", func(t *testing.T) {
		var hf SubHelmfileSpec
		require.NoError(t, yaml.Unmarshal([]byte(`myapp.yaml`), &hf))
		assert.Equal(t, "myapp.yaml", hf.Path)
		assert.Nil(t, hf.Inherits)
	})

	t.Run("no inherits leaves it nil", func(t *testing.T) {
		var hf SubHelmfileSpec
		require.NoError(t, yaml.Unmarshal([]byte("path: myapp.yaml\n"), &hf))
		assert.Nil(t, hf.Inherits)
	})
}

func TestSubHelmfileSpec_RejectsUnknownInheritsKey(t *testing.T) {
	var hf SubHelmfileSpec
	err := yaml.Unmarshal([]byte(`
path: myapp.yaml
inherits:
- repositories
- bunkKey
`), &hf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid inherits entry")
	assert.Contains(t, err.Error(), "bunkKey")
}

func TestSubHelmfileSpec_RejectsInheritsWithoutPath(t *testing.T) {
	var hf SubHelmfileSpec
	err := yaml.Unmarshal([]byte(`
inherits:
- repositories
`), &hf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "found 'inherits' definition without path")
}

func TestSubHelmfileSpec_AllAllowedKeysAccepted(t *testing.T) {
	for _, key := range AllowedInherits() {
		var hf SubHelmfileSpec
		err := yaml.Unmarshal([]byte("path: x.yaml\ninherits:\n- "+key+"\n"), &hf)
		require.NoErrorf(t, err, "key %q should be valid", key)
		assert.Equal(t, []string{key}, hf.Inherits)
	}
}

func TestAllowedInherits_DefensiveCopy(t *testing.T) {
	orig := AllowedInherits()
	require.NotEmpty(t, orig)

	// Mutating the returned slice must not affect validation, which backs onto
	// the unexported allowedInherits.
	mutated := AllowedInherits()
	mutated[0] = "tampered"

	// A genuinely valid key is still valid, the tampered value is not, and a fresh
	// call still returns the pristine set.
	assert.True(t, IsValidInherit("repositories"))
	assert.False(t, IsValidInherit("tampered"))
	assert.Equal(t, orig, AllowedInherits())
}

func TestSubHelmfileSpec_MarshalRoundTripInherits(t *testing.T) {
	hf := SubHelmfileSpec{
		Path:     "myapp.yaml",
		Inherits: []string{"repositories", "environments"},
	}
	out, err := yaml.Marshal(hf)
	require.NoError(t, err)

	var got SubHelmfileSpec
	require.NoError(t, yaml.Unmarshal(out, &got))
	assert.Equal(t, hf.Inherits, got.Inherits)
	assert.Equal(t, hf.Path, got.Path)
}
