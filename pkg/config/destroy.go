package config

// DestroyOptions is the options for the build command
type DestroyOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// SkipDeps is the skip deps flag
	SkipDeps bool
	// SkipCharts makes Destroy skip `withPreparedCharts`
	SkipCharts bool
}

// NewDestroyOptions creates a new Apply
func NewDestroyOptions() *DestroyOptions {
	return &DestroyOptions{}
}

// DestroyImpl is impl for applyOptions
type DestroyImpl struct {
	*GlobalImpl
	*DestroyOptions
}

// NewDestroyImpl creates a new DestroyImpl
func NewDestroyImpl(g *GlobalImpl, b *DestroyOptions) *DestroyImpl {
	return &DestroyImpl{
		GlobalImpl:     g,
		DestroyOptions: b,
	}
}

// Concurrency returns the concurrency
func (c *DestroyImpl) Concurrency() int {
	return c.DestroyOptions.Concurrency
}

// SkipDeps returns the skip deps
func (c *DestroyImpl) SkipDeps() bool {
	return c.GlobalOptions.SkipDeps || c.DestroyOptions.SkipDeps
}

// SkipCharts returns skipCharts flag
func (c *DestroyImpl) SkipCharts() bool {
	return c.DestroyOptions.SkipCharts
}
