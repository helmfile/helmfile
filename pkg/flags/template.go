package flags

import "github.com/spf13/cobra"

// TemplateFlagRegistrar handles flags specific to the template command
type TemplateFlagRegistrar struct {
    *GenericFlagRegistrar
    IncludeCRDs bool
}

// NewTemplateFlagRegistrar creates a new TemplateFlagRegistrar
func NewTemplateFlagRegistrar() *TemplateFlagRegistrar {
    return &TemplateFlagRegistrar{
        GenericFlagRegistrar: NewGenericFlagRegistrar(),
    }
}

// RegisterFlags registers template-specific flags
func (r *TemplateFlagRegistrar) RegisterFlags(cmd *cobra.Command) {
    r.RegisterBoolFlag(cmd, "include-crds", &r.IncludeCRDs, false, "include CRDs in the diffing")
}

