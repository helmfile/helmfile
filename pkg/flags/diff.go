package flags

import "github.com/spf13/cobra"

// DiffFlagRegistrar handles flags specific to the diff command
type DiffFlagRegistrar struct {
	*GenericFlagRegistrar
	IncludeCRDs bool
}

// NewDiffFlagRegistrar creates a new DiffFlagRegistrar
func NewDiffFlagRegistrar() *DiffFlagRegistrar {
	return &DiffFlagRegistrar{
		GenericFlagRegistrar: NewGenericFlagRegistrar(),
	}
}

// RegisterFlags registers diff-specific flags
func (r *DiffFlagRegistrar) RegisterFlags(cmd *cobra.Command) {
	r.RegisterBoolFlag(cmd, "include-crds", &r.IncludeCRDs, false, "include CRDs in the diffing")
	// Diff doesn't have skip-crds
}
