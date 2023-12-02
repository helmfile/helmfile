package tmpl

import (
	"github.com/helmfile/helmfile/pkg/filesystem"
)

type Context struct {
	preRender bool
	basePath  string
	// directory of the value of --file/-f option
	rootDir string
	fs      *filesystem.FileSystem
}

// SetBasePath sets the base path for the template
func (c *Context) SetBasePath(path string) {
	c.basePath = path
}

func (c *Context) SetFileSystem(fs *filesystem.FileSystem) {
	c.fs = fs
}
