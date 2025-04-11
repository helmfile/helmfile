package flags

import "github.com/spf13/cobra"

// SyncFlagRegistrar handles flags specific to the sync command
type SyncFlagRegistrar struct {
	*GenericFlagRegistrar
	IncludeCRDs bool
	SkipCRDs    bool
}

// NewSyncFlagRegistrar creates a new SyncFlagRegistrar
func NewSyncFlagRegistrar() *SyncFlagRegistrar {
	return &SyncFlagRegistrar{
		GenericFlagRegistrar: NewGenericFlagRegistrar(),
	}
}

// RegisterFlags registers sync-specific flags
func (r *SyncFlagRegistrar) RegisterFlags(cmd *cobra.Command) {
	r.RegisterBoolFlag(cmd, "include-crds", &r.IncludeCRDs, false, "include CRDs in the diffing")
	r.RegisterBoolFlag(cmd, "skip-crds", &r.SkipCRDs, false, "if set, no CRDs will be installed on sync. By default, CRDs are installed if not already present")
}
