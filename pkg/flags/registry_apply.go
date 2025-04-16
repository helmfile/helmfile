package flags

import "github.com/spf13/cobra"

// ApplyFlagRegistry handles flags specific to the apply command
type ApplyFlagRegistry struct {
	*GenericFlagRegistry
	IncludeCRDs bool
	SkipCRDs    bool
}

// NewApplyFlagRegistry creates a new ApplyFlagRegistry
func NewApplyFlagRegistry() *ApplyFlagRegistry {
	return &ApplyFlagRegistry{
		GenericFlagRegistry: NewGenericFlagRegistry(),
	}
}

// RegisterFlags registers apply-specific flags
func (r *ApplyFlagRegistry) RegisterFlags(cmd *cobra.Command) {
	r.RegisterBoolFlag(cmd, "include-crds", &r.IncludeCRDs, false, "include CRDs in the diffing")
	r.RegisterBoolFlag(cmd, "skip-crds", &r.SkipCRDs, false, "if set, no CRDs will be installed on sync. By default, CRDs are installed if not already present")
}
