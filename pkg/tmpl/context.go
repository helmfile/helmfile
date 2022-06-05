package tmpl

type Context struct {
	preRender bool
	basePath  string
	readFile  func(string) ([]byte, error)
}

// SetBasePath sets the base path for the template
func (c *Context) SetBasePath(path string) {
	c.basePath = path
}

func (c *Context) SetReadFile(f func(string) ([]byte, error)) {
	c.readFile = f
}
