package config

// LintImpl is impl for LintOptions
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
