package config

import (
	"fmt"
	"os"
	"strings"
)

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

// SkipCRDs returns the skip crds
func (t *TemplateImpl) SkipCRDs() bool {
	return t.TemplateOptions.SkipCRDsFlag.Value()
}

// IncludeCRDs returns the include crds
func (t *TemplateImpl) IncludeCRDs() bool {
	return t.TemplateOptions.IncludeCRDsFlag.Value()
}

// ShouldIncludeCRDs determines if CRDs should be included in the operation.
func (t *TemplateImpl) ShouldIncludeCRDs() bool {
	return ShouldIncludeCRDs(t.IncludeCRDsFlag, t.SkipCRDsFlag)
}

// NoHooks returns the no hooks
func (t *TemplateImpl) NoHooks() bool {
	return t.TemplateOptions.NoHooks
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

// PostRendererArgs returns the PostRendererArgs.
func (t *TemplateImpl) PostRendererArgs() []string {
	return t.TemplateOptions.PostRendererArgs
}

// SkipSchemaValidation returns the SkipSchemaValidation.
func (t *TemplateImpl) SkipSchemaValidation() bool {
	return t.TemplateOptions.SkipSchemaValidation
}

// KubeVersion returns the the KubeVersion.
func (t *TemplateImpl) KubeVersion() string {
	return t.TemplateOptions.KubeVersion
}

// ShowOnly returns the ShowOnly.
func (t *TemplateImpl) ShowOnly() []string {
	return t.TemplateOptions.ShowOnly
}
