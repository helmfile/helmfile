package config

import (
	"fmt"
	"strings"
)

type CreateOptions struct {
	Name      string
	OutputDir string
	Force     bool
}

func NewCreateOptions() *CreateOptions {
	return &CreateOptions{}
}

type CreateImpl struct {
	*GlobalImpl
	*CreateOptions
}

func NewCreateImpl(g *GlobalImpl, o *CreateOptions) *CreateImpl {
	return &CreateImpl{
		GlobalImpl:    g,
		CreateOptions: o,
	}
}

func (c *CreateImpl) Name() string {
	return c.CreateOptions.Name
}

func (c *CreateImpl) OutputDir() string {
	if c.CreateOptions.OutputDir != "" {
		return c.CreateOptions.OutputDir
	}
	if c.CreateOptions.Name != "" {
		return c.CreateOptions.Name
	}
	return "."
}

func (c *CreateImpl) Force() bool {
	return c.CreateOptions.Force
}

func (c *CreateImpl) ValidateConfig() error {
	name := c.CreateOptions.Name
	if name != "" {
		if strings.ContainsAny(name, "/\\") {
			return fmt.Errorf("invalid project name %q: must not contain path separators", name)
		}
		if name == ".." || name == "." {
			return fmt.Errorf("invalid project name %q", name)
		}
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("project name must not be empty or whitespace only")
		}
	}
	return c.GlobalImpl.ValidateConfig()
}
