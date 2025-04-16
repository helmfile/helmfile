package config

// DestroyImpl is impl for DestroyOptions
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
