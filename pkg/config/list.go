package config

// ListOptions is the options for the build command
type ListOptions struct {
	// Output is the output format
	Output string
	// KeepTempDir is the keep temp dir flag
	KeepTempDir bool
	// WithPreparedCharts makes list call `withPreparedCharts` when listing
	WithPreparedCharts bool
}

// NewListOptions creates a new Apply
func NewListOptions() *ListOptions {
	return &ListOptions{}
}

// ListImpl is impl for applyOptions
type ListImpl struct {
	*GlobalImpl
	*ListOptions
}

// NewListImpl creates a new ListImpl
func NewListImpl(g *GlobalImpl, b *ListOptions) *ListImpl {
	return &ListImpl{
		GlobalImpl:  g,
		ListOptions: b,
	}
}

// Args returns the args
func (c *ListImpl) Args() string {
	return ""
}

// Output returns the output
func (c *ListImpl) Output() string {
	return c.ListOptions.Output
}

// WithPreparedCharts returns withPreparedCharts flag
func (c *ListImpl) WithPreparedCharts() bool {
	return c.ListOptions.WithPreparedCharts
}
