package config

import (
	"github.com/helmfile/helmfile/pkg/common"
	"github.com/helmfile/helmfile/pkg/flags"
)

// DiffOptions is the options for the build command
type DiffOptions struct {
	// Set is the set flag
	Set []string
	// Values is the values flag
	Values []string
	// DetailedExitcode is the detailed exit code
	DetailedExitcode bool
	// StripTrailingCR is true if trailing carriage returns should be stripped during diffing
	StripTrailingCR bool
	// IncludeTests is the include tests flag
	IncludeTests bool
	// SkipCRDsFlag is the skip crds flag
	SkipCRDsFlag common.BoolFlag
	// IncludeCRDsFlag is the include crds flag
	IncludeCRDsFlag common.BoolFlag
	// SkipNeeds is the include crds flag
	SkipNeeds bool
	// IncludeNeeds is the include needs flag
	IncludeNeeds bool
	// IncludeTransitiveNeeds is the include transitive needs flag
	IncludeTransitiveNeeds bool
	// SkipDiffOnInstall is the skip diff on install flag
	SkipDiffOnInstall bool
	// ShowSecrets is the show secrets flag
	ShowSecrets bool
	// NoHooks skips hooks during diff
	NoHooks bool
	// Suppress is the suppress flag
	Suppress []string
	// SuppressSecrets is the suppress secrets flag
	SuppressSecrets bool
	// Concurrency is the concurrency flag
	Concurrency int
	// Validate is the validate flag
	Validate bool
	// Context is the context flag
	Context int
	// Output is output flag
	Output string
	// ReuseValues is true if the helm command should reuse the values
	ReuseValues bool
	// ResetValues is true if helm command should reset values to charts' default
	ResetValues bool
	// Propagate '--post-renderer' to helmv3 template and helm install
	PostRenderer string
	// Propagate '--post-renderer-args' to helmv3 template and helm install
	PostRendererArgs []string
	// DiffArgs is the list of arguments to pass to helm-diff.
	DiffArgs string
	// SuppressOutputLineRegex is a list of regexes to suppress output lines
	SuppressOutputLineRegex []string
	SkipSchemaValidation    bool
}

// NewDiffOptions creates a new Apply
func NewDiffOptions() *DiffOptions {
	newOptions := &DiffOptions{}
	newOptions.Initialize()

	return newOptions
}

func (o *DiffOptions) Initialize() {
	flags.EnsureBoolFlag(&o.IncludeCRDsFlag, false)
	flags.EnsureBoolFlag(&o.SkipCRDsFlag, false) // not exposed as cli flag but needed for ShouldIncludeCRDs() until skip-crds is removed
}

func (o *DiffOptions) HandleFlag(name string, value interface{}, changed bool) bool {
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
