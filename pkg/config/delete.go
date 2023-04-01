// TODO: Remove this function once Helmfile v0.x
package config

// DeleteOptions is the options for the build command
type DeleteOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// Purge is the purge flag
	Purge bool
	// SkipCharts makes Delete skip `withPreparedCharts`
	SkipCharts bool
}

// NewDeleteOptions creates a new Apply
func NewDeleteOptions() *DeleteOptions {
	return &DeleteOptions{}
}

// DeleteImpl is impl for applyOptions
type DeleteImpl struct {
	*GlobalImpl
	*DeleteOptions
}

// NewDeleteImpl creates a new DeleteImpl
func NewDeleteImpl(g *GlobalImpl, b *DeleteOptions) *DeleteImpl {
	return &DeleteImpl{
		GlobalImpl:    g,
		DeleteOptions: b,
	}
}

// Concurrency returns the concurrency
func (c *DeleteImpl) Concurrency() int {
	return c.DeleteOptions.Concurrency
}

// Purge returns the purge
func (c *DeleteImpl) Purge() bool {
	return c.DeleteOptions.Purge
}

// SkipDeps returns the skip deps
func (c *DeleteImpl) SkipDeps() bool {
	return c.GlobalOptions.SkipDeps
}

// SkipCharts returns skipCharts flag
func (c *DeleteImpl) SkipCharts() bool {
	return c.DeleteOptions.SkipCharts
}
