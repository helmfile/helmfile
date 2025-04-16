package config

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

// Output returns the output
func (c *ListImpl) Output() string {
	return c.ListOptions.Output
}

// SkipCharts returns skipCharts flag
func (c *ListImpl) SkipCharts() bool {
	return c.ListOptions.SkipCharts
}
