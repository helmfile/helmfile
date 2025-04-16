package config

// InitImpl is impl for InitOptions
type InitImpl struct {
	*GlobalImpl
	*InitOptions
}

// NewInitImpl creates a new InitImpl
func NewInitImpl(g *GlobalImpl, b *InitOptions) *InitImpl {
	return &InitImpl{
		GlobalImpl:  g,
		InitOptions: b,
	}
}

// Force returns the Force.
func (b *InitImpl) Force() bool {
	return b.InitOptions.Force
}
