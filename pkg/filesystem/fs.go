package filesystem

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type fileStat struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (fs fileStat) Name() string       { return fs.name }
func (fs fileStat) Size() int64        { return fs.size }
func (fs fileStat) Mode() fs.FileMode  { return fs.mode }
func (fs fileStat) ModTime() time.Time { return fs.modTime }
func (fs fileStat) IsDir() bool        { return fs.mode.IsDir() }
func (fs fileStat) Sys() any           { return nil }

type FileSystem struct {
	ReadFile          func(string) ([]byte, error)
	ReadDir           func(string) ([]fs.DirEntry, error)
	DeleteFile        func(string) error
	FileExists        func(string) (bool, error)
	Glob              func(string) ([]string, error)
	FileExistsAt      func(string) bool
	DirectoryExistsAt func(string) bool
	Stat              func(string) (os.FileInfo, error)
	Getwd             func() (string, error)
	Chdir             func(string) error
	Abs               func(string) (string, error)
	EvalSymlinks      func(string) (string, error)
}

func DefaultFileSystem() *FileSystem {
	dfs := FileSystem{
		ReadDir:      os.ReadDir,
		DeleteFile:   os.Remove,
		Stat:         os.Stat,
		Glob:         filepath.Glob,
		Getwd:        os.Getwd,
		Chdir:        os.Chdir,
		EvalSymlinks: filepath.EvalSymlinks,
	}

	dfs.Stat = dfs.stat
	dfs.ReadFile = dfs.readFile
	dfs.FileExistsAt = dfs.fileExistsAtDefault
	dfs.DirectoryExistsAt = dfs.directoryExistsDefault
	dfs.FileExists = dfs.fileExistsDefault
	dfs.Abs = dfs.absDefault
	return &dfs
}

func FromFileSystem(params FileSystem) *FileSystem {
	dfs := DefaultFileSystem()

	if params.ReadFile != nil {
		dfs.ReadFile = params.ReadFile
	}
	if params.ReadDir != nil {
		dfs.ReadDir = params.ReadDir
	}
	if params.DeleteFile != nil {
		dfs.DeleteFile = params.DeleteFile
	}
	if params.FileExists != nil {
		dfs.FileExists = params.FileExists
	}
	if params.Glob != nil {
		dfs.Glob = params.Glob
	}
	if params.FileExistsAt != nil {
		dfs.FileExistsAt = params.FileExistsAt
	}
	if params.DirectoryExistsAt != nil {
		dfs.DirectoryExistsAt = params.DirectoryExistsAt
	}
	if params.Stat != nil {
		dfs.Stat = params.Stat
	}
	if params.Getwd != nil {
		dfs.Getwd = params.Getwd
	}
	if params.Chdir != nil {
		dfs.Chdir = params.Chdir
	}
	if params.Abs != nil {
		dfs.Abs = params.Abs
	}
	if params.EvalSymlinks != nil {
		dfs.EvalSymlinks = params.EvalSymlinks
	}
	return dfs
}

func (filesystem *FileSystem) stat(name string) (os.FileInfo, error) {
	if name == "-" {
		return fileStat{mode: 0}, nil
	}
	return os.Stat(name)
}

func (filesystem *FileSystem) readFile(name string) ([]byte, error) {
	if name == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(name)
}

func (filesystem *FileSystem) fileExistsAtDefault(path string) bool {
	path, err := filesystem.resolveSymlinks(path)
	if err != nil {
		return false
	}
	fileInfo, err := filesystem.Stat(path)
	return err == nil && fileInfo.Mode().IsRegular()
}

func (filesystem *FileSystem) fileExistsDefault(path string) (bool, error) {
	path, err := filesystem.resolveSymlinks(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	_, err = filesystem.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (filesystem *FileSystem) directoryExistsDefault(path string) bool {
	path, err := filesystem.resolveSymlinks(path)
	if err != nil {
		return false
	}
	fileInfo, err := filesystem.Stat(path)
	return err == nil && fileInfo.Mode().IsDir()
}

func (filesystem *FileSystem) resolveSymlinks(path string) (string, error) {
	if !filepath.IsAbs(path) && !filepath.IsLocal(path) {
		basePath, err := filesystem.Getwd()
		if err != nil {
			return "", err
		}

		basePath, err = filesystem.EvalSymlinks(basePath)
		if err != nil {
			return "", err
		}
		path, err := filesystem.EvalSymlinks(filepath.Join(basePath, path))
		if err != nil {
			return "", err
		}
		return path, nil
	}
	return path, nil
}

func (filesystem *FileSystem) absDefault(path string) (string, error) {
	path, err := filesystem.resolveSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}
