package flags

import "github.com/spf13/cobra"

// FlagRegistry defines an interface for registering and transferring flags
type FlagRegistry interface {
	RegisterFlags(cmd *cobra.Command)
	TransferFlags(cmd *cobra.Command, opts interface{})
	GetRegisteredFlagNames() []string
	GetValues() map[string]interface{}
	IsFlagRegistered(name string) bool
}

// GenericFlagRegistry is a base struct for flag registries
type GenericFlagRegistry struct {
	// Map of flag names to their values
	values map[string]interface{}
}

// NewGenericFlagRegistry creates a new GenericFlagRegistrar
func NewGenericFlagRegistry() *GenericFlagRegistry {
	return &GenericFlagRegistry{
		values: make(map[string]interface{}),
	}
}

// GetValues returns the internal values map
func (r *GenericFlagRegistry) GetValues() map[string]interface{} {
	return r.values
}

// GetRegisteredFlagNames returns the names of all registered flags
func (r *GenericFlagRegistry) GetRegisteredFlagNames() []string {
	names := make([]string, 0, len(r.values))
	for name := range r.values {
		names = append(names, name)
	}
	return names
}

// IsFlagRegistered checks if a flag is registered in the registry
func (r *GenericFlagRegistry) IsFlagRegistered(name string) bool {
	_, exists := r.values[name]
	return exists
}

// TransferFlags transfers all registered flags to the options
func (r *GenericFlagRegistry) TransferFlags(cmd *cobra.Command, opts interface{}) {
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
func (r *GenericFlagRegistry) RegisterBoolFlag(cmd *cobra.Command, name string, value *bool, defaultValue bool, usage string) {
	cmd.Flags().BoolVar(value, name, defaultValue, usage)
	r.values[name] = value
}
