package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
)

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
}

func DefaultFileSystem() *FileSystem {
	dfs := FileSystem{
		ReadFile:   os.ReadFile,
		ReadDir:    os.ReadDir,
		DeleteFile: os.Remove,
		Stat:       os.Stat,
		Glob:       filepath.Glob,
		Getwd:      os.Getwd,
		Chdir:      os.Chdir,
		Abs:        filepath.Abs,
	}

	dfs.FileExistsAt = dfs.fileExistsAtDefault
	dfs.DirectoryExistsAt = dfs.directoryExistsDefault
	dfs.FileExists = dfs.fileExistsDefault
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

	return dfs
}

func (filesystem *FileSystem) fileExistsAtDefault(path string) bool {
	fileInfo, err := filesystem.Stat(path)
	return err == nil && fileInfo.Mode().IsRegular()
}

func (filesystem *FileSystem) fileExistsDefault(path string) (bool, error) {
	_, err := filesystem.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (filesystem *FileSystem) directoryExistsDefault(path string) bool {
	fileInfo, err := filesystem.Stat(path)
	return err == nil && fileInfo.Mode().IsDir()
}
