package config

// LintOptions is the options for the build command
type LintOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// SkipDeps is the skip deps flag
	SkipDeps bool
	// Set is the set flags to pass to helm lint
	Set []string
	// Values is the values flags to pass to helm lint
	Values []string
	// SkipNeeds is the skip needs flag
	SkipNeeds bool
	// IncludeNeeds is the include needs flag
	IncludeNeeds bool
	// IncludeTransitiveNeeds is the include transitive needs flag
	IncludeTransitiveNeeds bool
	// SkipDeps is the skip deps flag
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
func (l *LintImpl) Concurrency() int {
	return l.LintOptions.Concurrency
}

// SkipDeps returns the skip deps
func (l *LintImpl) SkipDeps() bool {
	return l.LintOptions.SkipDeps
}

// Set returns the Set
func (l *LintImpl) Set() []string {
	return l.LintOptions.Set
}

// Values returns the Values
func (l *LintImpl) Values() []string {
	return l.LintOptions.Values
}

// SkipCleanUp returns the skip clean up
func (l *LintImpl) SkipCleanup() bool {
	return false
}

// IncludeNeeds returns the include needs
func (l *LintImpl) IncludeNeeds() bool {
	return l.LintOptions.IncludeNeeds || l.IncludeTransitiveNeeds()
}

// IncludeTransitiveNeeds returns the include transitive needs
func (l *LintImpl) IncludeTransitiveNeeds() bool {
	return l.LintOptions.IncludeTransitiveNeeds
}

// SkipNeeds returns the skip needs
func (l *LintImpl) SkipNeeds() bool {
	if !l.IncludeNeeds() {
		return l.LintOptions.SkipNeeds
	}

	return false
}
