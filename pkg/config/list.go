package config

// ListOptions is the options for the build command
type ListOptions struct {
	// Output is the output format
	Output string
	// KeepTempDir is the keep temp dir flag
	KeepTempDir bool
	// SkipCharts makes List skip `withPreparedCharts`
	SkipCharts bool
}

// NewListOptions creates a new Apply
func NewListOptions() *ListOptions {
	return &ListOptions{}
}
