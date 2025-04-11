package flags

import "github.com/spf13/cobra"

// ApplyFlagRegistrar handles flags specific to the apply command
type ApplyFlagRegistrar struct {
    *GenericFlagRegistrar
    IncludeCRDs bool
    SkipCRDs    bool
}

// NewApplyFlagRegistrar creates a new ApplyFlagRegistrar
func NewApplyFlagRegistrar() *ApplyFlagRegistrar {
    return &ApplyFlagRegistrar{
        GenericFlagRegistrar: NewGenericFlagRegistrar(),
    }
}

// RegisterFlags registers apply-specific flags
func (r *ApplyFlagRegistrar) RegisterFlags(cmd *cobra.Command) {
    r.RegisterBoolFlag(cmd, "include-crds", &r.IncludeCRDs, false, "include CRDs in the diffing")
    r.RegisterBoolFlag(cmd, "skip-crds", &r.SkipCRDs, false, "if set, no CRDs will be installed on sync. By default, CRDs are installed if not already present")
}

