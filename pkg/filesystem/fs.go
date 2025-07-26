package filesystem

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
	Dir               func(string) string
	Stat              func(string) (os.FileInfo, error)
	Getwd             func() (string, error)
	Chdir             func(string) error
	Abs               func(string) (string, error)
	EvalSymlinks      func(string) (string, error)
	CopyDir           func(src, dst string) error
}

func DefaultFileSystem() *FileSystem {
	dfs := FileSystem{
		ReadDir:      os.ReadDir,
		DeleteFile:   os.Remove,
		Glob:         filepath.Glob,
		Getwd:        os.Getwd,
		Chdir:        os.Chdir,
		EvalSymlinks: filepath.EvalSymlinks,
		Dir:          filepath.Dir,
	}

	dfs.Stat = dfs.stat
	dfs.ReadFile = dfs.readFile
	dfs.FileExistsAt = dfs.fileExistsAtDefault
	dfs.DirectoryExistsAt = dfs.directoryExistsDefault
	dfs.FileExists = dfs.fileExistsDefault
	dfs.Abs = dfs.absDefault
	dfs.CopyDir = dfs.copyDirDefault
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
	if params.Dir != nil {
		dfs.Dir = params.Dir
	}
	if params.CopyDir != nil {
		dfs.CopyDir = params.CopyDir
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

// copyDirDefault recursively copies a directory tree, preserving permissions.
func (filesystem *FileSystem) copyDirDefault(src string, dst string) error {
	src, err := filesystem.EvalSymlinks(src)
	if err != nil {
		return err
	}

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == src {
			return nil
		}

		if strings.HasPrefix(filepath.Base(path), ".") {
			// Skip any dot files
			if info.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}

		// The "path" has the src prefixed to it. We need to join our
		// destination with the path without the src on it.
		dstPath := filepath.Join(dst, path[len(src):])

		// we don't want to try and copy the same file over itself.
		if eq, err := SameFile(path, dstPath); eq {
			return nil
		} else if err != nil {
			return err
		}

		// If we have a directory, make that subdirectory, then continue
		// the walk.
		if info.IsDir() {
			if path == filepath.Join(src, dst) {
				// dst is in src; don't walk it.
				return nil
			}

			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}

			return nil
		}

		// If the current path is a symlink, recreate the symlink relative to
		// the dst directory
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}

			return os.Symlink(target, dstPath)
		}

		// If we have a file, copy the contents.
		srcF, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			_ = srcF.Close()
		}()

		dstF, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer func() {
			_ = dstF.Close()
		}()

		if _, err := io.Copy(dstF, srcF); err != nil {
			return err
		}

		// Chmod it
		return os.Chmod(dstPath, info.Mode())
	}

	return filepath.Walk(src, walkFn)
}

// SameFile returns true if the two given paths refer to the same physical
// file on disk, using the unique file identifiers from the underlying
// operating system. For example, on Unix systems this checks whether the
// two files are on the same device and have the same inode.
func SameFile(a, b string) (bool, error) {
	if a == b {
		return true, nil
	}

	aInfo, err := os.Lstat(a)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	bInfo, err := os.Lstat(b)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return os.SameFile(aInfo, bInfo), nil
}
