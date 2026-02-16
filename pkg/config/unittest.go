package config

// UnittestOptions is the options for the unittest command
type UnittestOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// Set is the set flags to pass to helm unittest
	Set []string
	// Values is the values flags to pass to helm unittest
	Values []string
	// FailFast causes helm-unittest to quit immediately when a test fails
	FailFast bool
	// Color enforces colored output even when stdout is not a tty
	Color bool
	// DebugPlugin enables verbose output from the helm-unittest plugin
	DebugPlugin bool
	// SkipNeeds is the skip needs flag
	SkipNeeds bool
	// IncludeNeeds is the include needs flag
	IncludeNeeds bool
	// IncludeTransitiveNeeds is the include transitive needs flag
	IncludeTransitiveNeeds bool
}

// NewUnittestOptions creates a new UnittestOptions
func NewUnittestOptions() *UnittestOptions {
	return &UnittestOptions{}
}

// UnittestImpl is impl for UnittestOptions
type UnittestImpl struct {
	*GlobalImpl
	*UnittestOptions
}

// NewUnittestImpl creates a new UnittestImpl
func NewUnittestImpl(g *GlobalImpl, u *UnittestOptions) *UnittestImpl {
	return &UnittestImpl{
		GlobalImpl:      g,
		UnittestOptions: u,
	}
}

// Concurrency returns the concurrency
func (u *UnittestImpl) Concurrency() int {
	return u.UnittestOptions.Concurrency
}

// Set returns the Set
func (u *UnittestImpl) Set() []string {
	return u.UnittestOptions.Set
}

// Values returns the Values
func (u *UnittestImpl) Values() []string {
	return u.UnittestOptions.Values
}

// FailFast returns the fail fast flag
func (u *UnittestImpl) FailFast() bool {
	return u.UnittestOptions.FailFast
}

// Color returns the color flag
func (u *UnittestImpl) Color() bool {
	return u.UnittestOptions.Color
}

// DebugPlugin returns the debug plugin flag
func (u *UnittestImpl) DebugPlugin() bool {
	return u.UnittestOptions.DebugPlugin
}

// SkipCleanup returns the skip clean up
func (u *UnittestImpl) SkipCleanup() bool {
	return false
}

// IncludeNeeds returns the include needs
func (u *UnittestImpl) IncludeNeeds() bool {
	return u.UnittestOptions.IncludeNeeds || u.IncludeTransitiveNeeds()
}

// IncludeTransitiveNeeds returns the include transitive needs
func (u *UnittestImpl) IncludeTransitiveNeeds() bool {
	return u.UnittestOptions.IncludeTransitiveNeeds
}

// SkipNeeds returns the skip needs
func (u *UnittestImpl) SkipNeeds() bool {
	if !u.IncludeNeeds() {
		return u.UnittestOptions.SkipNeeds
	}

	return false
}

// EnforceNeedsAreInstalled returns false for unittest
func (u *UnittestImpl) EnforceNeedsAreInstalled() bool {
	return false
}
