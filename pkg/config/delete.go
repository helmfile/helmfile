package config

// DeleteOptions is the options for the build command
type DeleteOptions struct {
	// Args is the args to pass to helm exec
	Args string
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// Purge is the purge flag
	Purge bool
	// SkipDeps is the skip deps flag
	SkipDeps bool
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

// Args returns the args
func (c *DeleteImpl) Args() string {
	return c.DeleteOptions.Args
}

// Purge returns the purge
func (c *DeleteImpl) Purge() bool {
	return c.DeleteOptions.Purge
}

// SkipDeps returns the skip deps
func (c *DeleteImpl) SkipDeps() bool {
	return c.DeleteOptions.SkipDeps
}
