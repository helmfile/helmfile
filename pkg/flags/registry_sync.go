package flags

import "github.com/spf13/cobra"

// SyncFlagRegistry handles flags specific to the sync command
type SyncFlagRegistry struct {
	*GenericFlagRegistry
	IncludeCRDs bool
	SkipCRDs    bool
}

// NewSyncFlagRegistry creates a new SyncFlagRegistry
func NewSyncFlagRegistry() *SyncFlagRegistry {
	return &SyncFlagRegistry{
		GenericFlagRegistry: NewGenericFlagRegistry(),
	}
}

// RegisterFlags registers sync-specific flags
func (r *SyncFlagRegistry) RegisterFlags(cmd *cobra.Command) {
	r.RegisterBoolFlag(cmd, "include-crds", &r.IncludeCRDs, false, "include CRDs in the diffing")
	r.RegisterBoolFlag(cmd, "skip-crds", &r.SkipCRDs, false, "if set, no CRDs will be installed on sync. By default, CRDs are installed if not already present")
}
