package filesystem

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"
)

type TestFileInfo struct {
	mode fs.FileMode
}

func (tfi TestFileInfo) Name() string       { return "" }
func (tfi TestFileInfo) Size() int64        { return 0 }
func (tfi TestFileInfo) Mode() fs.FileMode  { return tfi.mode }
func (tfi TestFileInfo) ModTime() time.Time { return time.Time{} }
func (tfi TestFileInfo) IsDir() bool        { return tfi.mode.IsDir() }
func (tfi TestFileInfo) Sys() any           { return nil }

func NewTestFileSystem() FileSystem {
	replaceffs := FileSystem{
		Stat: func(s string) (os.FileInfo, error) {
			if strings.HasPrefix(s, "existing_file") {
				return TestFileInfo{mode: 0}, nil
			}
			if strings.HasPrefix(s, "existing_dir") {
				return TestFileInfo{mode: fs.ModeDir}, nil
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
