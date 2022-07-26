package tmpl

import "io/fs"

type Context struct {
	preRender bool
	basePath  string
	readFile  func(string) ([]byte, error)
	readDir   func(string) ([]fs.DirEntry, error)
}

// SetBasePath sets the base path for the template
func (c *Context) SetBasePath(path string) {
	c.basePath = path
}

func (c *Context) SetReadFile(f func(string) ([]byte, error)) {
	c.readFile = f
}

func (c *Context) SetReadDir(f func(string) ([]fs.DirEntry, error)) {
	c.readDir = f
}
