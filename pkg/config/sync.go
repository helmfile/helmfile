package config

import (
	"github.com/helmfile/helmfile/pkg/common"
	"github.com/helmfile/helmfile/pkg/flags"
)

// SyncOptions is the options for the build command
type SyncOptions struct {
	// Set is the set flag
	Set []string
	// Values is the values flag
	Values []string
	// Concurrency is the concurrency flag
	Concurrency int
	// Validate is the validate flag
	Validate bool
	// IncludeCRDs is the include crds flag
	SkipNeeds bool
	// IncludeNeeds is the include needs flag
	IncludeNeeds bool
	// IncludeTransitiveNeeds is the include transitive needs flag
	IncludeTransitiveNeeds bool
	// SkipCRDsFlag is the skip crds flag
	SkipCRDsFlag common.BoolFlag
	// IncludeCRDsFlag is the include crds flag
	IncludeCRDsFlag common.BoolFlag
	// Wait is the wait flag
	Wait bool
	// WaitRetries is the wait retries flag
	WaitRetries int
	// WaitForJobs is the wait for jobs flag
	WaitForJobs bool
	// ReuseValues is true if the helm command should reuse the values
	ReuseValues bool
	// ResetValues is true if helm command should reset values to charts' default
	ResetValues bool
	// Propagate '--post-renderer' to helmv3 template and helm install
	PostRenderer string
	// Propagate '--post-renderer-args' to helmv3 template and helm install
	PostRendererArgs []string
	// Propagate '--skip-schema-validation' to helmv3 template and helm install
	SkipSchemaValidation bool
	// Cascade '--cascade' to helmv3 delete, available values: background, foreground, or orphan, default: background
	Cascade string
	// SyncArgs is the list of arguments to pass to the helm upgrade command.
	SyncArgs string
	// HideNotes is the hide notes flag
	HideNotes bool
	// TakeOwnership is the take ownership flag
	TakeOwnership bool
	// SyncReleaseLabels is the sync release labels flag
	SyncReleaseLabels bool
}

// NewSyncOptions creates a new SyncOption
func NewSyncOptions() *SyncOptions {
	newOptions := &SyncOptions{}
	newOptions.Initialize()

	return newOptions
}

func (o *SyncOptions) Initialize() {
	flags.EnsureBoolFlag(&o.IncludeCRDsFlag, false)
	flags.EnsureBoolFlag(&o.SkipCRDsFlag, false)
}

func (o *SyncOptions) HandleFlag(name string, value interface{}, changed bool) bool {
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
