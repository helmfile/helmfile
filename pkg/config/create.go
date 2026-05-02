package config

import (
	"fmt"
	"os"
	"path/filepath"
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
		if name == ".." {
			return fmt.Errorf("invalid project name %q", name)
		}
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("project name must not be empty or whitespace only")
		}
	}
	outputDir := c.OutputDir()
	absDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("failed to resolve output directory: %w", err)
	}
	if !c.Force() {
		scaffoldPaths := []string{
			filepath.Join(absDir, "helmfile.yaml"),
			filepath.Join(absDir, "environments", "default.yaml"),
			filepath.Join(absDir, "values", ".gitkeep"),
		}
		var existing []string
		for _, p := range scaffoldPaths {
			if _, err := os.Stat(p); err == nil {
				existing = append(existing, p)
			}
		}
		if len(existing) > 0 {
			return fmt.Errorf("the following files already exist, use --force to overwrite: %s", strings.Join(existing, ", "))
		}
	}
	return c.GlobalImpl.ValidateConfig()
}
