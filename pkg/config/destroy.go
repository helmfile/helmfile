package config

// DestroyOptions is the options for the build command
type DestroyOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// SkipCharts makes Destroy skip `withPreparedCharts`
	SkipCharts bool
	// Cascade '--cascade' to helmv3 delete, available values: background, foreground, or orphan, default: background
	Cascade string
	// Wait is the wait flag
	Wait bool
	// Wait '--wait' if set, will wait until all the resources are destroyed before returning. It will wait for as long as --timeout
	DeleteWait bool
	// Timeout '--timeout', to wait for helm operation (default 5m0s)
	DeleteTimeout int
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

// SkipCharts returns skipCharts flag
func (c *DestroyImpl) SkipCharts() bool {
	return c.DestroyOptions.SkipCharts
}

// Cascade returns cascade flag
func (c *DestroyImpl) Cascade() string {
	return c.DestroyOptions.Cascade
}

// DeleteWait returns the wait flag
func (c *DestroyImpl) DeleteWait() bool {
	return c.DestroyOptions.DeleteWait
}

// DeleteTimeout returns the timeout flag
func (c *DestroyImpl) DeleteTimeout() int {
	return c.DestroyOptions.DeleteTimeout
}
