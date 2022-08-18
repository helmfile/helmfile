package testhelper

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"
	"time"
)

type TestFs struct {
	Cwd string

	GlobFixtures map[string][]string

	fileReaderCalls int
	successfulReads []string

	mapFS fstest.MapFS
}

func NewTestFs(files map[string]string) *TestFs {
	dirs := map[string]bool{}
	for abs := range files {
		for d := filepath.ToSlash(filepath.Dir(abs)); !dirs[d]; d = filepath.ToSlash(filepath.Dir(d)) {
			dirs[d] = true
			fmt.Fprintf(os.Stderr, "testfs: recognized dir: %s\n", d)
		}
	}

	mapFS := fstest.MapFS{}
	t := time.Now()
	for k, v := range files {
		if k[0] == '/' {
			k = k[1:]
		}
		mapFS[k] = &fstest.MapFile{
			Data:    []byte(v),
			Mode:    0755,
			ModTime: t,
		}
	}

	return &TestFs{
		Cwd:   "/path/to",
		mapFS: mapFS,

		successfulReads: []string{},

		GlobFixtures: map[string][]string{},
	}
}

func (f *TestFs) FileExistsAt(path string) bool {
	info, _ := f.mapFS.Stat(f.transPath(path))
	return info != nil && !info.IsDir()
}

func (f *TestFs) FileExists(path string) (bool, error) {
	return f.FileExistsAt(path), nil
}

func (f *TestFs) DirectoryExistsAt(path string) bool {
	info, _ := f.mapFS.Stat(f.transPath(path))
	return info != nil && info.IsDir()
}

func (f *TestFs) ReadFile(filename string) ([]byte, error) {
	data, err := f.mapFS.ReadFile(f.transPath(filename))
	if err != nil {
		return data, err
	}

	f.fileReaderCalls++

	f.successfulReads = append(f.successfulReads, filename)

	return data, nil
}

func (f *TestFs) ReadDir(path string) ([]fs.DirEntry, error) {
	return f.mapFS.ReadDir(f.transPath(path))
}

func (f *TestFs) SuccessfulReads() []string {
	return f.successfulReads
}

func (f *TestFs) FileReaderCalls() int {
	return f.fileReaderCalls
}

func (f *TestFs) Glob(relPattern string) ([]string, error) {
	res, err := f.mapFS.Glob(f.transPath(relPattern))
	if err != nil {
		return nil, err
	}

	var files []string
	for _, f := range res {
		files = append(files, "/"+f)
	}
	return files, nil
}

func (f *TestFs) Abs(path string) (string, error) {
	return "/" + filepath.ToSlash(filepath.Clean(f.transPath(path))), nil
}

func (f *TestFs) transPath(p string) string {
	if !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, f.Cwd[1:]) {
		p = filepath.ToSlash(filepath.Join(f.Cwd, p))
	}
	if p[0] == '/' {
		p = p[1:]
	}

	return p
}

func (f *TestFs) Getwd() (string, error) {
	return f.Cwd, nil
}

func (f *TestFs) Chdir(dir string) error {
	if f.DirectoryExistsAt(dir) {
		f.Cwd = dir
		return nil
	}
	return fmt.Errorf("unexpected chdir \"%s\"", dir)
}
