package flags

import "github.com/spf13/cobra"

// DiffFlagRegistry handles flags specific to the diff command
type DiffFlagRegistry struct {
	*GenericFlagRegistry
	IncludeCRDs bool
}

// NewDiffFlagRegistry creates a new DiffFlagRegistry
func NewDiffFlagRegistry() *DiffFlagRegistry {
	return &DiffFlagRegistry{
		GenericFlagRegistry: NewGenericFlagRegistry(),
	}
}

// RegisterFlags registers diff-specific flags
func (r *DiffFlagRegistry) RegisterFlags(cmd *cobra.Command) {
	r.RegisterBoolFlag(cmd, "include-crds", &r.IncludeCRDs, false, "include CRDs in the diffing")
	// Diff doesn't have skip-crds
}
