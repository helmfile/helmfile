package config

// LintOptions is the options for the build command
type LintOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// SkipDeps is the skip deps flag
	SkipDeps bool
	// Args is the args to pass to helm lint
	Args string
	// Set is the set flags to pass to helm lint
	Set []string
	// Values is the values flags to pass to helm lint
	Values []string
}

// NewLintOptions creates a new Apply
func NewLintOptions() *LintOptions {
	return &LintOptions{}
}

// LintImpl is impl for applyOptions
type LintImpl struct {
	*GlobalImpl
	*LintOptions
}

// NewLintImpl creates a new LintImpl
func NewLintImpl(g *GlobalImpl, b *LintOptions) *LintImpl {
	return &LintImpl{
		GlobalImpl:  g,
		LintOptions: b,
	}
}

// Concurrency returns the concurrency
func (c *LintImpl) Concurrency() int {
	return c.LintOptions.Concurrency
}

// SkipDeps returns the skip deps
func (c *LintImpl) SkipDeps() bool {
	return c.LintOptions.SkipDeps
}

// Args returns the args
func (c *LintImpl) Args() string {
	return c.LintOptions.Args
}

// Set returns the Set
func (c *LintImpl) Set() []string {
	return c.LintOptions.Set
}

// Values returns the Values
func (c *LintImpl) Values() []string {
	return c.LintOptions.Values
}

// SkipCleanUp returns the skip clean up
func (c *LintImpl) SkipCleanup() bool {
	return false
}

// SkipNeeds returns the skip needs
func (c *LintImpl) SkipNeeds() bool {
	return false
}

// IncludeNeeds returns the include needs
func (c *LintImpl) IncludeNeeds() bool {
	return false
}

// IncludeTransitiveNeeds returns the include transitive needs
func (c *LintImpl) IncludeTransitiveNeeds() bool {
	return false
}
