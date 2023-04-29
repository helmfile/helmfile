package config

import (
	"fmt"
	"os"
	"strings"
)

// TemplateOptions is the options for the build command
type TemplateOptions struct {
	// Set is the set flag
	Set []string
	// Values is the values flag
	Values []string
	// OutputDir is the output dir flag
	OutputDir string
	// OutputDirTemplate is the output dir template flag
	OutputDirTemplate string
	// Concurrency is the concurrency flag
	Concurrency int
	// Validate is the validate flag
	Validate bool
	// IncludeCRDs is the include crds flag
	IncludeCRDs bool
	// SkipTests is the skip tests flag
	SkipTests bool
	// SkipNeeds is the skip needs flag
	SkipNeeds bool
	// IncludeNeeds is the include needs flag
	IncludeNeeds bool
	// IncludeTransitiveNeeds is the include transitive needs flag
	IncludeTransitiveNeeds bool
	// SkipCleanup is the skip cleanup flag
	SkipCleanup bool
	// Propagate '--post-renderer' to helmv3 template and helm install
	PostRenderer string
	// KubeVersion is the kube-version flag
	KubeVersion string
}

// NewTemplateOptions creates a new Apply
func NewTemplateOptions() *TemplateOptions {
	return &TemplateOptions{}
}

// TemplateImpl is impl for applyOptions
type TemplateImpl struct {
	*GlobalImpl
	*TemplateOptions
}

// NewTemplateImpl creates a new TemplateImpl
func NewTemplateImpl(g *GlobalImpl, t *TemplateOptions) *TemplateImpl {
	return &TemplateImpl{
		GlobalImpl:      g,
		TemplateOptions: t,
	}
}

// Concurrency returns the concurrency
func (t *TemplateImpl) Concurrency() int {
	return t.TemplateOptions.Concurrency
}

// IncludeCRDs returns the include crds
func (t *TemplateImpl) IncludeCRDs() bool {
	return t.TemplateOptions.IncludeCRDs
}

// IncludeNeeds returns the include needs
func (t *TemplateImpl) IncludeNeeds() bool {
	return t.TemplateOptions.IncludeNeeds || t.IncludeTransitiveNeeds()
}

// IncludeTransitiveNeeds returns the include transitive needs
func (t *TemplateImpl) IncludeTransitiveNeeds() bool {
	return t.TemplateOptions.IncludeTransitiveNeeds
}

// OutputDir returns the output dir
func (t *TemplateImpl) OutputDir() string {
	return strings.TrimRight(t.TemplateOptions.OutputDir, fmt.Sprintf("%c", os.PathSeparator))
}

// OutputDirTemplate returns the output dir template
func (t *TemplateImpl) OutputDirTemplate() string {
	return t.TemplateOptions.OutputDirTemplate
}

// Set returns the Set
func (t *TemplateImpl) Set() []string {
	return t.TemplateOptions.Set
}

// SkipCleanup returns the skip cleanup
func (t *TemplateImpl) SkipCleanup() bool {
	return t.TemplateOptions.SkipCleanup
}

// SkipNeeds returns the skip needs
func (t *TemplateImpl) SkipNeeds() bool {
	if !t.IncludeNeeds() {
		return t.TemplateOptions.SkipNeeds
	}

	return false
}

// SkipTests returns the skip tests
func (t *TemplateImpl) SkipTests() bool {
	return t.TemplateOptions.SkipTests
}

// Validate returns the validate
func (t *TemplateImpl) Validate() bool {
	return t.TemplateOptions.Validate
}

// Values returns the values
func (t *TemplateImpl) Values() []string {
	return t.TemplateOptions.Values
}

// PostRenderer returns the PostRenderer.
func (t *TemplateImpl) PostRenderer() string {
	return t.TemplateOptions.PostRenderer
}

// KubeVersion returns the the KubeVersion.
func (t *TemplateImpl) KubeVersion() string {
	return t.TemplateOptions.KubeVersion
}
