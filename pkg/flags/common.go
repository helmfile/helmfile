package flags

import (
    "github.com/spf13/cobra"
)

// FlagRegistrar defines an interface for registering and transferring flags
type FlagRegistrar interface {
    RegisterFlags(cmd *cobra.Command)
    TransferFlags(cmd *cobra.Command, opts interface{})
}

// FlagHandler is a generic interface for handling flag values
type FlagHandler interface {
    // HandleFlag receives a flag name, value, and whether it was changed
    HandleFlag(name string, value interface{}, changed bool)
}

// GenericFlagRegistrar is a base struct for flag registrars
type GenericFlagRegistrar struct {
    // Map of flag names to their values
    values map[string]interface{}
}

// NewGenericFlagRegistrar creates a new GenericFlagRegistrar
func NewGenericFlagRegistrar() *GenericFlagRegistrar {
    return &GenericFlagRegistrar{
        values: make(map[string]interface{}),
    }
}

// TransferFlags transfers all registered flags to the options
func (r *GenericFlagRegistrar) TransferFlags(cmd *cobra.Command, opts interface{}) {
    if handler, ok := opts.(FlagHandler); ok {
        flags := cmd.Flags()

        // Transfer each registered flag
        for name, value := range r.values {
            changed := flags.Changed(name)
            handler.HandleFlag(name, value, changed)
        }
    }
}

// RegisterBoolFlag registers a boolean flag and stores its reference
func (r *GenericFlagRegistrar) RegisterBoolFlag(cmd *cobra.Command, name string, value *bool, defaultValue bool, usage string) {
    cmd.Flags().BoolVar(value, name, defaultValue, usage)
    r.values[name] = value
}
