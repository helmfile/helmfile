package config

// StatusOptions is the options for the build command
type StatusOptions struct {
	// Concurrency is the concurrent flag
	Concurrency int
}

// NewStatusOptions creates a new Apply
func NewStatusOptions() *StatusOptions {
	return &StatusOptions{}
}

// StatusImpl is impl for applyOptions
type StatusImpl struct {
	*GlobalImpl
	*StatusOptions
}

// NewStatusImpl creates a new StatusImpl
func NewStatusImpl(g *GlobalImpl, b *StatusOptions) *StatusImpl {
	return &StatusImpl{
		GlobalImpl:    g,
		StatusOptions: b,
	}
}

// IncludeTransitiveNeeds returns the include transitive needs
func (s *StatusImpl) IncludeTransitiveNeeds() bool {
	return false
}

// Concurrency returns the concurrency
func (s *StatusImpl) Concurrency() int {
	return s.StatusOptions.Concurrency
}
