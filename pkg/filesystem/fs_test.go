package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	ffs := NewTestFileSystem()
	path, err := ffs.resolveSymlinks("../existing_file.txt")
	require.NoErrorf(t, err, "Expected no error but got %v", err)
	require.Equalf(t, "/real/existing_file.txt", path, "Expected absolute path %s but got %s", "/real/existing_file.txt", path)

	path, err = ffs.resolveSymlinks("../missing_file.txt")
	require.ErrorIsf(t, err, os.ErrExist, "Expected error %v but got %v", os.ErrExist, err)
	require.Equalf(t, "", path, "Expected empty path but got %s", path)

	path, err = ffs.resolveSymlinks("./existing_file.txt")
	require.NoErrorf(t, err, "Expected no error but got %v", err)
	require.Equalf(t, "./existing_file.txt", path, "Expected local path %s but got %s", "./existing_file.txt", path)

	path, err = ffs.resolveSymlinks("existing_file.txt")
	require.NoErrorf(t, err, "Expected no error but got %v", err)
	require.Equalf(t, "existing_file.txt", path, "Expected local path %s but got %s", "existing_file.txt", path)

	path, err = ffs.resolveSymlinks("/a/b/c/existing_file.txt")

	require.NoErrorf(t, err, "Expected no error but got %v", err)
	require.Equalf(t, "/a/b/c/existing_file.txt", path, "Expected absolute path %s but got %s", "/a/b/c/existing_file.txt", path)
}

func TestFs_fileExistsDefault(t *testing.T) {
	ffs := NewTestFileSystem()
	exists, err := ffs.FileExists("existing_file.txt")
	require.NoErrorf(t, err, "Expected no error but got %v", err)
	require.Truef(t, exists, "Expected file %s, not found", "existing_file.txt")

	exists, err = ffs.FileExists("missing_file.txt")
	require.Falsef(t, exists, "Not expected file %s, found", "missing_file.txt")
	require.ErrorIsf(t, err, os.ErrExist, "Expected error %v but got %v", os.ErrExist, err)

	dfs := DefaultFileSystem()
	exists, err = dfs.FileExists("-")
	require.NoErrorf(t, err, "Expected no error but got %v", err)
	require.Truef(t, exists, "Expected file %s, not found", "-")
}

func TestFs_fileExistsAtDefault(t *testing.T) {
	ffs := NewTestFileSystem()

	exists := ffs.FileExistsAt("existing_file.txt")
	require.Truef(t, exists, "Expected file %s, not found", "existing_file.txt")

	exists = ffs.FileExistsAt("missing_file.txt")
	require.Falsef(t, exists, "Not expected file %s, found", "missing_file.txt")

	exists = ffs.FileExistsAt("existing_dir")
	require.Falsef(t, exists, "Not expected file %s, found", "existing_dir")

	dfs := DefaultFileSystem()
	exists = dfs.FileExistsAt("-")
	require.Truef(t, exists, "Expected file %s, not found", "-")
}

func TestFs_directoryExistsDefault(t *testing.T) {
	ffs := NewTestFileSystem()
	exists := ffs.DirectoryExistsAt("existing_dir")
	require.Truef(t, exists, "Expected file %s, not found", "existing_dir")

	exists = ffs.DirectoryExistsAt("missing_dir")
	require.Falsef(t, exists, "Not expected file %s, found", "missing_dir")
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
			if err != nil {
				t.Errorf("create file %s error: %v", yamlPath, err)
			}
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
	if ffs.ReadFile == nil ||
		ffs.ReadDir == nil ||
		ffs.DeleteFile == nil ||
		ffs.FileExists == nil ||
		ffs.Glob == nil ||
		ffs.FileExistsAt == nil ||
		ffs.DirectoryExistsAt == nil ||
		ffs.Stat == nil ||
		ffs.Getwd == nil ||
		ffs.Chdir == nil ||
		ffs.Abs == nil ||
		ffs.EvalSymlinks == nil {
		t.Errorf("Missing functions in DefaultFileSystem")
	}
}
