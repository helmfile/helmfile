package tmpl

import (
	"github.com/helmfile/helmfile/pkg/filesystem"
)

type Context struct {
	preRender bool
	basePath  string
	fs        *filesystem.FileSystem

	// lenientRequiredEnv, when true, makes requiredEnv return an empty string
	// instead of failing when the environment variable is unset or empty.
	// This is used to allow selector-based filtering to skip releases whose
	// requiredEnv calls would otherwise fail during whole-document rendering.
	// See https://github.com/helmfile/helmfile/issues/1172
	lenientRequiredEnv bool

	// missingRequiredEnvs records the names of environment variables that
	// were unset/empty and returned as empty strings due to lenientRequiredEnv.
	missingRequiredEnvs []string
}

// SetBasePath sets the base path for the template
func (c *Context) SetBasePath(path string) {
	c.basePath = path
}

func (c *Context) SetFileSystem(fs *filesystem.FileSystem) {
	c.fs = fs
}

// GetMissingRequiredEnvs returns the names of environment variables that were
// unset/empty and returned as empty strings due to lenientRequiredEnv.
func (c *Context) GetMissingRequiredEnvs() []string {
	if len(c.missingRequiredEnvs) == 0 {
		return nil
	}
	result := make([]string, len(c.missingRequiredEnvs))
	copy(result, c.missingRequiredEnvs)
	return result
}
