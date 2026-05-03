package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCreateImplWithDefaults(name string) *CreateImpl {
	return NewCreateImpl(NewGlobalImpl(&GlobalOptions{}), &CreateOptions{
		Name: name,
	})
}

func TestCreateImpl_ValidateConfig_NameWithForwardSlash(t *testing.T) {
	c := newTestCreateImplWithDefaults("foo/bar")
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain path separators")
}

func TestCreateImpl_ValidateConfig_NameWithBackslash(t *testing.T) {
	c := newTestCreateImplWithDefaults(`foo\bar`)
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain path separators")
}

func TestCreateImpl_ValidateConfig_NameDotDot(t *testing.T) {
	c := newTestCreateImplWithDefaults("..")
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid project name")
}

func TestCreateImpl_ValidateConfig_NameDot(t *testing.T) {
	c := newTestCreateImplWithDefaults(".")
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid project name")
}

func TestCreateImpl_ValidateConfig_WhitespaceOnlyName(t *testing.T) {
	c := newTestCreateImplWithDefaults("   ")
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty or whitespace only")
}

func TestCreateImpl_ValidateConfig_ValidName(t *testing.T) {
	c := newTestCreateImplWithDefaults("myproject")
	require.NoError(t, c.ValidateConfig())
}

func TestCreateImpl_ValidateConfig_GlobalColorConflict(t *testing.T) {
	// Delegates to GlobalImpl.ValidateConfig which rejects --color + --no-color.
	c := NewCreateImpl(
		NewGlobalImpl(&GlobalOptions{Color: true, NoColor: true}),
		&CreateOptions{},
	)
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--color")
	assert.Contains(t, err.Error(), "--no-color")
}
