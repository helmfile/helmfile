package config

// DepsOptions is the options for the build command
type DepsOptions struct {
	// Args is the args to pass to helm exec
	Args string
	// SkipRepos is the skip repos flag
	SkipRepos bool
	// Concurrency is the maximum number of concurrent helm processes to run
	Concurrency int
}

// NewDepsOptions creates a new Apply
func NewDepsOptions() *DepsOptions {
	return &DepsOptions{}
}

// DepsImpl is impl for applyOptions
type DepsImpl struct {
	*GlobalImpl
	*DepsOptions
}

// NewDepsImpl creates a new DepsImpl
func NewDepsImpl(g *GlobalImpl, b *DepsOptions) *DepsImpl {
	return &DepsImpl{
		GlobalImpl:  g,
		DepsOptions: b,
	}
}

// Args returns the args
func (d *DepsImpl) Args() string {
	return d.DepsOptions.Args
}

// SkipDeps returns the skip deps
func (d *DepsImpl) SkipRepos() bool {
	return d.DepsOptions.SkipRepos
}

// IncludeTransitiveNeeds returns the includeTransitiveNeeds
func (d *DepsImpl) IncludeTransitiveNeeds() bool {
	return false
}

// Concurrency returns the concurrency
func (c *DepsImpl) Concurrency() int {
	return c.DepsOptions.Concurrency
}
