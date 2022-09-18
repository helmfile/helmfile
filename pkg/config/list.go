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

// SkipCharts returns skipCharts flag
func (c *ListImpl) SkipCharts() bool {
	return c.ListOptions.SkipCharts
}
