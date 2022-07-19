package config

// ReposOptions is the options for the build command
type ReposOptions struct {
	// Args is the args
	Args string
}

// NewReposOptions creates a new Apply
func NewReposOptions() *ReposOptions {
	return &ReposOptions{}
}

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

// Args returns the args
func (r *ReposImpl) Args() string {
	return r.ReposOptions.Args
}

// IncludeTransitiveNeeds returns the include transitive needs
func (r *ReposImpl) IncludeTransitiveNeeds() bool {
	return false
}
