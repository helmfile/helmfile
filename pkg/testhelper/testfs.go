package testhelper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	ffs "github.com/helmfile/helmfile/pkg/filesystem"
)

type TestFileInfo struct {
	name    string
	mode    os.FileMode
	size    int64
	modTime time.Time
}

func (f TestFileInfo) Name() string { return f.name }
func (f TestFileInfo) Size() int64  { return f.size }
func (f TestFileInfo) Mode() os.FileMode {
	return f.mode
}
func (f TestFileInfo) ModTime() time.Time { return f.modTime }
func (f TestFileInfo) IsDir() bool {
	return f.Mode() == os.ModeDir
}

func (f TestFileInfo) Sys() interface{} {
	return nil
}

type TestFs struct {
	Cwd   string
	dirs  map[string]bool
	files map[string]string

	GlobFixtures map[string][]string
	DeleteFile   func(string) error

	fileReaderCalls int
	successfulReads []string
}

func NewTestFs(files map[string]string) *TestFs {
	dirs := map[string]bool{}
	for abs := range files {
		for d := filepath.ToSlash(filepath.Dir(abs)); !dirs[d]; d = filepath.ToSlash(filepath.Dir(d)) {
			dirs[d] = true
		}
	}
	return &TestFs{
		Cwd:   "/path/to",
		dirs:  dirs,
		files: files,

		successfulReads: []string{},

		GlobFixtures: map[string][]string{},
		DeleteFile:   func(string) (ret error) { return },
	}
}

func (f *TestFs) ToFileSystem() *ffs.FileSystem {
	curfs := ffs.FileSystem{
		FileExistsAt:      f.FileExistsAt,
		FileExists:        f.FileExists,
		DirectoryExistsAt: f.DirectoryExistsAt,
		ReadFile:          f.ReadFile,
		Glob:              f.Glob,
		Getwd:             f.Getwd,
		Chdir:             f.Chdir,
		Abs:               f.Abs,
		DeleteFile:        f.DeleteFile,
		Stat:              f.Stat,
	}
	trfs := ffs.FromFileSystem(curfs)
	return trfs
}

func (f *TestFs) FileExistsAt(path string) bool {
	var ok bool
	if strings.HasPrefix(path, "/") {
		_, ok = f.files[path]
	} else {
		_, ok = f.files[filepath.ToSlash(filepath.Join(f.Cwd, path))]
	}
	return ok
}

func (f *TestFs) FileExists(path string) (bool, error) {
	return f.FileExistsAt(path), nil
}

func (f *TestFs) DirectoryExistsAt(path string) bool {
	var ok bool
	if strings.HasPrefix(path, "/") {
		_, ok = f.dirs[path]
	} else {
		_, ok = f.dirs[filepath.ToSlash(filepath.Join(f.Cwd, path))]
	}
	return ok
}

func (f *TestFs) Stat(path string) (os.FileInfo, error) {
	if _, ok := f.dirs[path]; ok {
		return TestFileInfo{
			name: path,
			mode: os.ModeDir,
		}, nil
	}

	if _, ok := f.files[path]; ok {
		return TestFileInfo{
			name: path,
			mode: os.ModePerm,
		}, nil
	}
	return nil, fmt.Errorf("%s does not exist", path)
}

func (f *TestFs) ReadFile(filename string) ([]byte, error) {
	var str string
	var ok bool
	if strings.HasPrefix(filename, "/") {
		str, ok = f.files[filename]
	} else {
		str, ok = f.files[filepath.ToSlash(filepath.Join(f.Cwd, filename))]
	}
	if !ok {
		return []byte(nil), os.ErrNotExist
	}

	f.fileReaderCalls++

	f.successfulReads = append(f.successfulReads, filename)

	return []byte(str), nil
}

func (f *TestFs) SuccessfulReads() []string {
	return f.successfulReads
}

func (f *TestFs) FileReaderCalls() int {
	return f.fileReaderCalls
}

func (f *TestFs) Glob(relPattern string) ([]string, error) {
	var pattern string
	if strings.HasPrefix(relPattern, "/") {
		pattern = relPattern
	} else {
		pattern = filepath.ToSlash(filepath.Join(f.Cwd, relPattern))
	}

	fixtures, ok := f.GlobFixtures[pattern]
	if ok {
		return fixtures, nil
	}

	matches := []string{}
	for name := range f.files {
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			return nil, err
		}
		if matched {
			matches = append(matches, name)
		}
	}
	return matches, nil
}

func (f *TestFs) Abs(path string) (string, error) {
	path = filepath.ToSlash(path)
	var p string
	if strings.HasPrefix(path, "/") {
		p = path
	} else {
		p = filepath.Join(f.Cwd, path)
	}
	return filepath.ToSlash(filepath.Clean(p)), nil
}

func (f *TestFs) Getwd() (string, error) {
	return f.Cwd, nil
}

func (f *TestFs) Chdir(dir string) error {
	if _, ok := f.dirs[dir]; ok {
		f.Cwd = dir
		return nil
	}
	return fmt.Errorf("unexpected chdir \"%s\"", dir)
}
