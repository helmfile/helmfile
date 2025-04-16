package flags

import "github.com/spf13/cobra"

// MockFlagRegistry implements FlagRegistrar for testing
type MockFlagRegistry struct {
	*GenericFlagRegistry
}

func NewMockFlagRegistry() *MockFlagRegistry {
	return &MockFlagRegistry{
		GenericFlagRegistry: NewGenericFlagRegistry(),
	}
}

// RegisterFlags implements the FlagRegistrar interface for testing
func (r *MockFlagRegistry) RegisterFlags(cmd *cobra.Command) {
	// Mock implementation does nothing
}

// GetValues returns the internal values map
func (r *MockFlagRegistry) GetValues() map[string]interface{} {
	return r.values
}
