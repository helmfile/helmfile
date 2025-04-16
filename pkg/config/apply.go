package config

import (
	"github.com/helmfile/helmfile/pkg/common"
	"github.com/helmfile/helmfile/pkg/flags"
)

// ApplyOptoons is the options for the apply command
type ApplyOptions struct {
	// Set is a list of key value pairs to be merged into the command
	Set []string
	// Values is a list of value files to be merged into the command
	Values []string
	// Concurrency is the maximum number of concurrent helm processes to run
	Concurrency int
	// Validate is validate your manifests against the Kubernetes cluster you are currently pointing at. Note that this requires access to a Kubernetes cluster to obtain information necessary for validating, like the list of available API versions
	Validate bool
	// Context is the number of lines of context to show around changes
	Context int
	// Output is the output format for the diff plugin
	Output string
	// DetailedExitcode is true if the exit code should be 2 instead of 0 if there were changes detected and the changes were synced successfully
	DetailedExitcode bool
	// StripTrailingCR is true if trailing carriage returns should be stripped during diffing
	StripTrailingCR bool
	// SkipCleanup is true if the cleanup of temporary values files should be skipped
	SkipCleanup bool
	// SkipCRDsFlag is true if the CRDs should be skipped
	SkipCRDsFlag common.BoolFlag
	// IncludeCRDsFlag is true if the CRDs should be included
	IncludeCRDsFlag common.BoolFlag
	// SkipNeeds is true if the needs should be skipped
	SkipNeeds bool
	// IncludeNeeds is true if the needs should be included
	IncludeNeeds bool
	// IncludeTransitiveNeeds is true if the transitive needs should be included
	IncludeTransitiveNeeds bool
	// SkipDiffOnInstall is true if the diff should be skipped on install
	SkipDiffOnInstall bool
	// DiffArgs is the list of arguments to pass to the helm-diff.
	DiffArgs string
	// IncludeTests is true if the tests should be included
	IncludeTests bool
	// Suppress is true if the output should be suppressed
	Suppress []string
	// SuppressSecrets is true if the secrets should be suppressed
	SuppressSecrets bool
	// ShowSecrets is true if the secrets should be shown
	ShowSecrets bool
	// NoHooks skips checking for hooks
	NoHooks bool
	// SuppressDiff is true if the diff should be suppressed
	SuppressDiff bool
	// Wait is true if the helm command should wait for the release to be deployed
	Wait bool
	// WaitRetries is the number of times to retry waiting for the release to be deployed
	WaitRetries int
	// WaitForJobs is true if the helm command should wait for the jobs to be completed
	WaitForJobs bool
	// Propagate '--skip-schema-validation' to helmv3 template and helm install
	SkipSchemaValidation bool
	// ReuseValues is true if the helm command should reuse the values
	ReuseValues bool
	// ResetValues is true if helm command should reset values to charts' default
	ResetValues bool
	// Propagate '--post-renderer' to helmv3 template and helm install
	PostRenderer string
	// Propagate '--post-renderer-args' to helmv3 template and helm install
	PostRendererArgs []string
	// Cascade '--cascade' to helmv3 delete, available values: background, foreground, or orphan, default: background
	Cascade string
	// SuppressOutputLineRegex is a list of regexes to suppress output lines
	SuppressOutputLineRegex []string
	// SyncArgs is the list of arguments to pass to helm upgrade.
	SyncArgs string
	// HideNotes is the hide notes flag
	HideNotes bool

	// TakeOwnership is true if the ownership should be taken
	TakeOwnership bool

	SyncReleaseLabels bool
}

// NewApply creates a new Apply
func NewApplyOptions() *ApplyOptions {
	newOptions := &ApplyOptions{}
	newOptions.Initialize()

	return newOptions
}

func (o *ApplyOptions) Initialize() {
	flags.EnsureBoolFlag(&o.IncludeCRDsFlag, false)
	flags.EnsureBoolFlag(&o.SkipCRDsFlag, false)
}

func (o *ApplyOptions) HandleFlag(name string, value interface{}, changed bool) bool {
	switch name {
	case "include-crds":
		if changed {
			if boolVal, ok := value.(*bool); ok {
				o.IncludeCRDsFlag.Set(*boolVal)
			}
		}
		return true
	case "skip-crds":
		if changed {
			if boolVal, ok := value.(*bool); ok {
				o.SkipCRDsFlag.Set(*boolVal)
			}
		}
		return true
	}

	return false
}
