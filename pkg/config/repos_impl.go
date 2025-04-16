package config

// ReposImpl is impl for applyOptions
type ReposImpl struct {
	*GlobalImpl
	*ReposOptions
}

// NewReposImpl creates a new ReposImpl
func NewReposImpl(g *GlobalImpl, b *ReposOptions) *ReposImpl {
	return &ReposImpl{
		GlobalImpl:   g,
		ReposOptions: b,
	}
}

// IncludeTransitiveNeeds returns the include transitive needs
func (r *ReposImpl) IncludeTransitiveNeeds() bool {
	return false
}
