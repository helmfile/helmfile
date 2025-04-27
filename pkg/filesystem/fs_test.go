package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func NewTestFileSystem() FileSystem {
	replaceffs := FileSystem{
		Stat: func(s string) (os.FileInfo, error) {
			if strings.HasSuffix(s, "existing_file.txt") {
				return fileStat{mode: 0}, nil
			}
			if strings.HasSuffix(s, "existing_dir") {
				return fileStat{mode: fs.ModeDir}, nil
			}
			return nil, os.ErrExist
		},
		Getwd: func() (string, error) {
			return "/test/dir", nil
		},
		EvalSymlinks: func(s string) (string, error) {
			if s == "/test/dir" {
				return "/real/dir", nil
			} else {
				if strings.Contains(s, "missing") {
					return "", os.ErrExist
				}
				return s, nil
			}
		},
	}
	return *FromFileSystem(replaceffs)
}

func TestFs_resolveSymlinks(t *testing.T) {
	cases := []struct {
		name        string
		symlinkPath string
		expected    string
		expectedErr error
	}{
		{
			name:        "existing file symlink",
			symlinkPath: "../existing_file.txt",
			expected:    "/real/existing_file.txt",
			expectedErr: nil,
		},
		{
			name:        "missing file symlink",
			symlinkPath: "../missing_file.txt",
			expected:    "",
			expectedErr: os.ErrExist,
		},
		{
			name:        "local existing file symlink",
			symlinkPath: "./existing_file.txt",
			expected:    "./existing_file.txt",
			expectedErr: nil,
		},
		{
			name:        "absolute existing file symlink",
			symlinkPath: "/a/b/c/existing_file.txt",
			expected:    "/a/b/c/existing_file.txt",
			expectedErr: nil,
		},
		{
			name:        "current directory existing file symlink",
			symlinkPath: "existing_file.txt",
			expected:    "existing_file.txt",
			expectedErr: nil,
		},
	}

	ffs := NewTestFileSystem()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path, err := ffs.resolveSymlinks(c.symlinkPath)
			if c.expectedErr != nil {
				require.ErrorIs(t, err, c.expectedErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, c.expected, path)
		})
	}
}

func TestFs_fileExistsDefault(t *testing.T) {
	cases := []struct {
		name        string
		filePath    string
		expected    bool
		expectedErr error
	}{
		{
			name:        "existing file",
			filePath:    "existing_file.txt",
			expected:    true,
			expectedErr: nil,
		},
		{
			name:        "missing file",
			filePath:    "missing_file.txt",
			expected:    false,
			expectedErr: os.ErrExist,
		},
	}

	ffs := NewTestFileSystem()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exists, err := ffs.FileExists(c.filePath)
			if c.expectedErr != nil {
				require.ErrorIs(t, err, c.expectedErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, c.expected, exists)
		})
	}
}

func TestFs_fileExistsAtDefault(t *testing.T) {
	cases := []struct {
		name     string
		filePath string
		expected bool
	}{
		{
			name:     "existing file",
			filePath: "existing_file.txt",
			expected: true,
		},
		{
			name:     "missing file",
			filePath: "missing_file.txt",
			expected: false,
		},
		{
			name:     "existing dir",
			filePath: "existing_dir",
			expected: false,
		},
	}

	ffs := NewTestFileSystem()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exists := ffs.FileExistsAt(c.filePath)
			assert.Equal(t, c.expected, exists)
		})
	}
}

func TestFs_directoryExistsDefault(t *testing.T) {
	cases := []struct {
		name     string
		dirPath  string
		expected bool
	}{
		{
			name:     "existing dir",
			dirPath:  "existing_dir",
			expected: true,
		},
		{
			name:     "missing dir",
			dirPath:  "missing_dir",
			expected: false,
		},
	}

	ffs := NewTestFileSystem()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exists := ffs.DirectoryExistsAt(c.dirPath)
			assert.Equal(t, c.expected, exists)
		})
	}
}

func TestFsTeadFile(t *testing.T) {
	cases := []struct {
		name      string
		content   []byte
		path      string
		wantError string
	}{
		{
			name:    "read file",
			content: []byte("hello helmfile"),
			path:    "helmfile.yaml",
		},
		{
			name:    "read file from stdin",
			content: []byte("hello helmfile"),
			path:    "-",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			yamlPath := filepath.Join(dir, c.path)

			dfs := DefaultFileSystem()
			tmpfile, err := os.Create(yamlPath)
			require.NoErrorf(t, err, "create file %s error: %v", yamlPath, err)

			_, err = tmpfile.Write(c.content)
			require.NoErrorf(t, err, "write to file %s error: %v", yamlPath, err)

			readPath := yamlPath
			if c.path == "-" {
				readPath = c.path
				oldOsStdin := os.Stdin
				defer func() { os.Stdin = oldOsStdin }()
				os.Stdin = tmpfile
			}
			_, err = tmpfile.Seek(0, 0)
			require.NoErrorf(t, err, "file %s seek error: %v", yamlPath, err)

			want, err := dfs.readFile(readPath)
			require.NoErrorf(t, err, "read file %s error: %v", readPath, err)
			require.Equalf(t, string(c.content), string(want), "unexpected error: unexpected=%s, got=%v", string(c.content), string(want))
		})
	}
}

func TestFs_DefaultBuilder(t *testing.T) {
	ffs := DefaultFileSystem()
	assert.NotNil(t, ffs.ReadFile)
	assert.NotNil(t, ffs.ReadDir)
	assert.NotNil(t, ffs.DeleteFile)
	assert.NotNil(t, ffs.FileExists)
	assert.NotNil(t, ffs.Glob)
	assert.NotNil(t, ffs.FileExistsAt)
	assert.NotNil(t, ffs.DirectoryExistsAt)
	assert.NotNil(t, ffs.Stat)
	assert.NotNil(t, ffs.Getwd)
	assert.NotNil(t, ffs.Chdir)
	assert.NotNil(t, ffs.Abs)
	assert.NotNil(t, ffs.EvalSymlinks)
}
