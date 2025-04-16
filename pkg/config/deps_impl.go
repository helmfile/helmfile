package config

// DepsImpl is impl for DepsOptions
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

// SkipRepos returns the skip deps
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
