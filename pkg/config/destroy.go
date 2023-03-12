package config

// DestroyOptions is the options for the build command
type DestroyOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// SkipDeps is the skip deps flag
	SkipDeps bool
	// SkipCharts makes Destroy skip `withPreparedCharts`
	SkipCharts bool
	// WaitReleaseAvailable is true if the helm command should wait for the release to not be in a pending state
	WaitReleaseAvailable bool
	// WaitReleaseTimeout is the timeout for the helm command to wait for the release to not be in a pending state
	WaitReleaseTimeout int
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
	return c.DestroyOptions.SkipDeps
}

// SkipCharts returns skipCharts flag
func (c *DestroyImpl) SkipCharts() bool {
	return c.DestroyOptions.SkipCharts
}

// WaitReleaseAvailable returns the wait release available
func (c *DestroyImpl) WaitReleaseAvailable() bool {
	return c.DestroyOptions.WaitReleaseAvailable
}

// WaitReleaseTimeout returns the wait release timeout
func (c *DestroyImpl) WaitReleaseTimeout() int {
	return c.DestroyOptions.WaitReleaseTimeout
}
