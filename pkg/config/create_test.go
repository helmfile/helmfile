package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCreateImpl(name, outputDir string, force bool) *CreateImpl {
	return NewCreateImpl(NewGlobalImpl(&GlobalOptions{}), &CreateOptions{
		Name:      name,
		OutputDir: outputDir,
		Force:     force,
	})
}

func TestCreateImpl_ValidateConfig_NameWithForwardSlash(t *testing.T) {
	c := newTestCreateImpl("foo/bar", "", false)
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain path separators")
}

func TestCreateImpl_ValidateConfig_NameWithBackslash(t *testing.T) {
	c := newTestCreateImpl(`foo\bar`, "", false)
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain path separators")
}

func TestCreateImpl_ValidateConfig_NameDotDot(t *testing.T) {
	c := newTestCreateImpl("..", "", false)
	err := c.ValidateConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid project name")
}

func TestCreateImpl_ValidateConfig_ValidName(t *testing.T) {
	dir := t.TempDir()
	c := newTestCreateImpl("myproject", dir, false)
	// outputDir is set explicitly so no files will be checked under "myproject"
	require.NoError(t, c.ValidateConfig())
}

func TestCreateImpl_ValidateConfig_ExistingHelmfileYAMLNoForce(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "helmfile.yaml"), []byte("x"), 0o644))

	c := newTestCreateImpl("", dir, false)
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

	c := newTestCreateImpl("", dir, false)
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

	c := newTestCreateImpl("", dir, false)
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

	c := newTestCreateImpl("", dir, true)
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
