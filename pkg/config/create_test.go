package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCreateImplWithDefaults(name, outputDir string, force bool) *CreateImpl {
	return NewCreateImpl(NewGlobalImpl(&GlobalOptions{}), &CreateOptions{
		Name:      name,
		OutputDir: outputDir,
		Force:     force,
	})
}

func TestCreateImpl_ValidateConfig_NameWithForwardSlash(t *testing.T) {
	c := newTestCreateImplWithDefaults("foo/bar", "", false)
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain path separators")
}

func TestCreateImpl_ValidateConfig_NameWithBackslash(t *testing.T) {
	c := newTestCreateImplWithDefaults(`foo\bar`, "", false)
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain path separators")
}

func TestCreateImpl_ValidateConfig_NameDotDot(t *testing.T) {
	c := newTestCreateImplWithDefaults("..", "", false)
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid project name")
}

func TestCreateImpl_ValidateConfig_WhitespaceOnlyName(t *testing.T) {
	c := newTestCreateImplWithDefaults("   ", "", false)
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty or whitespace only")
}

func TestCreateImpl_ValidateConfig_ValidName(t *testing.T) {
	dir := t.TempDir()
	// outputDir points to an empty temp dir so no scaffold files exist and the
	// name validation succeeds cleanly.
	c := newTestCreateImplWithDefaults("myproject", dir, false)
	require.NoError(t, c.ValidateConfig())
}

func TestCreateImpl_ValidateConfig_ExistingHelmfileYAMLNoForce(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helmfile.yaml"), []byte("x"), 0o644))

	c := newTestCreateImplWithDefaults("", dir, false)
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exist")
	assert.Contains(t, err.Error(), "--force")
}

func TestCreateImpl_ValidateConfig_ExistingEnvDefaultYAMLNoForce(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "environments")
	require.NoError(t, os.MkdirAll(envDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(envDir, "default.yaml"), []byte("x"), 0o644))

	c := newTestCreateImplWithDefaults("", dir, false)
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exist")
	assert.Contains(t, err.Error(), "--force")
}

func TestCreateImpl_ValidateConfig_ExistingGitkeepNoForce(t *testing.T) {
	dir := t.TempDir()
	valuesDir := filepath.Join(dir, "values")
	require.NoError(t, os.MkdirAll(valuesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(valuesDir, ".gitkeep"), []byte(""), 0o644))

	c := newTestCreateImplWithDefaults("", dir, false)
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exist")
	assert.Contains(t, err.Error(), "--force")
}

func TestCreateImpl_ValidateConfig_ExistingFilesWithForce(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helmfile.yaml"), []byte("x"), 0o644))
	envDir := filepath.Join(dir, "environments")
	require.NoError(t, os.MkdirAll(envDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(envDir, "default.yaml"), []byte("x"), 0o644))

	c := newTestCreateImplWithDefaults("", dir, true)
	require.NoError(t, c.ValidateConfig())
}

func TestCreateImpl_ValidateConfig_GlobalColorConflict(t *testing.T) {
	// Delegates to GlobalImpl.ValidateConfig which rejects --color + --no-color.
	c := NewCreateImpl(
		NewGlobalImpl(&GlobalOptions{Color: true, NoColor: true}),
		&CreateOptions{OutputDir: t.TempDir()},
	)
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--color")
	assert.Contains(t, err.Error(), "--no-color")
}
