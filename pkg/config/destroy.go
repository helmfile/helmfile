package config

// DestroyOptions is the options for the build command
type DestroyOptions struct {
	// Args is the args to pass to helm exec
	Args string
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// SkipDeps is the skip deps flag
	SkipDeps bool
	// Interactive is true if the user should be prompted for input.
	Interactive bool
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

// Args returns the args
func (c *DestroyImpl) Args() string {
	return c.DestroyOptions.Args
}

// SkipDeps returns the skip deps
func (c *DestroyImpl) SkipDeps() bool {
	return c.DestroyOptions.SkipDeps
}

// Interactive returns the Interactive
func (c *DestroyImpl) Interactive() bool {
	return c.DestroyOptions.Interactive
}
