package config

// InitOptions is the options for the init command
type InitOptions struct {
	Force bool
}

// NewInitOptions creates a new InitOptions
func NewInitOptions() *InitOptions {
	return &InitOptions{}
}

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
