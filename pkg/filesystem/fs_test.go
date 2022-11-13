package filesystem

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"
)

func NewTestFileSystem() FileSystem {
	replaceffs := FileSystem{
		Stat: func(s string) (os.FileInfo, error) {
			if strings.HasPrefix(s, "existing_file") {
				return fileStat{mode: 0}, nil
			}
			if strings.HasPrefix(s, "existing_dir") {
				return fileStat{mode: fs.ModeDir}, nil
			}
			return nil, errors.New("Error")
		},
	}
	return *FromFileSystem(replaceffs)
}

func TestFs_fileExistsDefault(t *testing.T) {
	ffs := NewTestFileSystem()
	var exists, _ = ffs.FileExists("existing_file.txt")
	if !exists {
		t.Errorf("Expected file %s, not found", "existing_file.txt")
	}

	exists, _ = ffs.FileExists("non_existing_file.txt")
	if exists {
		t.Errorf("Not expected file %s, found", "non_existing_file.txt")
	}

	dfs := DefaultFileSystem()
	exists, _ = dfs.FileExists("-")
	if !exists {
		t.Errorf("Not expected file %s, not found", "-")
	}
}

func TestFs_fileExistsAtDefault(t *testing.T) {
	ffs := NewTestFileSystem()

	var exists = ffs.FileExistsAt("existing_file.txt")
	if !exists {
		t.Errorf("Expected file %s, not found", "existing_file.txt")
	}

	exists = ffs.FileExistsAt("non_existing_file.txt")
	if exists {
		t.Errorf("Not expected file %s, found", "non_existing_file.txt")
	}

	exists = ffs.FileExistsAt("existing_dir")
	if exists {
		t.Errorf("Not expected file %s, found", "existing_dir")
	}

	dfs := DefaultFileSystem()
	exists = dfs.FileExistsAt("-")
	if !exists {
		t.Errorf("Not expected file %s, not found", "-")
	}
}

func TestFs_directoryExistsDefault(t *testing.T) {
	ffs := NewTestFileSystem()
	var exists = ffs.DirectoryExistsAt("existing_dir")
	if !exists {
		t.Errorf("Expected file %s, not found", "existing_dir")
	}

	exists = ffs.DirectoryExistsAt("not_existing_dir")
	if exists {
		t.Errorf("Not expected file %s, found", "existing_dir")
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
			yamlPath := path.Join(dir, c.path)

			dfs := DefaultFileSystem()
			tmpfile, err := os.Create(yamlPath)
			if err != nil {
				t.Errorf("create file %s error: %v", yamlPath, err)
			}
			_, err = tmpfile.Write(c.content)
			if err != nil {
				t.Errorf(" write to file %s error: %v", yamlPath, err)
			}
			readPath := yamlPath
			if c.path == "-" {
				readPath = c.path
				oldOsStdin := os.Stdin
				defer func() { os.Stdin = oldOsStdin }()
				os.Stdin = tmpfile
			}
			if _, err = tmpfile.Seek(0, 0); err != nil {
				t.Errorf("file %s seek error: %v", yamlPath, err)
			}

			want, err := dfs.readFile(readPath)
			if err != nil {
				t.Errorf("read file %s error: %v", readPath, err)
			} else {
				if string(c.content) != string(want) {
					t.Errorf("nexpected error: unexpected=%s, got=%v", string(c.content), string(want))
				}
			}
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
		ffs.Abs == nil {
		t.Errorf("Missing functions in DefaultFileSystem")
	}
}
