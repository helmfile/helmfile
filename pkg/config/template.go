package config

import (
	"github.com/helmfile/helmfile/pkg/common"
	"github.com/helmfile/helmfile/pkg/flags"
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
	// SkipCRDsFlag is the skip crds flag
	// Deprecated: Use IncludeCRDsFlag instead
	SkipCRDsFlag common.BoolFlag
	// IncludeCRDsFlag is the include crds flag
	IncludeCRDsFlag common.BoolFlag
	// SkipTests is the skip tests flag
	SkipTests bool
	// SkipNeeds is the skip needs flag
	SkipNeeds bool
	// IncludeNeeds is the include needs flag
	IncludeNeeds bool
	// IncludeTransitiveNeeds is the include transitive needs flag
	IncludeTransitiveNeeds bool
	// No-Hooks is the no hooks flag
	NoHooks bool
	// SkipCleanup is the skip cleanup flag
	SkipCleanup bool
	// Propagate '--post-renderer' to helmv3 template and helm install
	PostRenderer string
	// Propagate '--post-renderer-args' to helmv3 template and helm install
	PostRendererArgs []string
	// Propagate '--skip-schema-validation' to helmv3 template and helm install
	SkipSchemaValidation bool
	// KubeVersion is the kube-version flag
	KubeVersion string
	// Propagate '--show-only` to helm template
	ShowOnly []string
}

// NewTemplateOptions creates a new TemplateOption
func NewTemplateOptions() *TemplateOptions {
	options := &TemplateOptions{}
	options.Initialize()
	return options
}

func (o *TemplateOptions) Initialize() {
	flags.EnsureBoolFlag(&o.IncludeCRDsFlag, false)
	flags.EnsureBoolFlag(&o.SkipCRDsFlag, false) // not exposed as cli flag but needed for ShouldIncludeCRDs() until skip-crds is removed
}

func (o *TemplateOptions) HandleFlag(name string, value interface{}, changed bool) bool {
	switch name {
	case "include-crds":
		if changed {
			if boolVal, ok := value.(*bool); ok {
				o.IncludeCRDsFlag.Set(*boolVal)
			}
		}
		return true
	}

	return false
}
