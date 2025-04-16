package flags

import "github.com/spf13/cobra"

// TemplateFlagRegistry handles flags specific to the template command
type TemplateFlagRegistry struct {
	*GenericFlagRegistry
	IncludeCRDs bool
}

// NewTemplateFlagRegistry creates a new TemplateFlagRegistry
func NewTemplateFlagRegistry() *TemplateFlagRegistry {
	return &TemplateFlagRegistry{
		GenericFlagRegistry: NewGenericFlagRegistry(),
	}
}

// RegisterFlags registers template-specific flags
func (r *TemplateFlagRegistry) RegisterFlags(cmd *cobra.Command) {
	r.RegisterBoolFlag(cmd, "include-crds", &r.IncludeCRDs, false, "include CRDs in the diffing")
	// Template doesn't have skip-crds
}
