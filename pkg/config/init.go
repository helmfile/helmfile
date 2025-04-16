package config

// InitOptions is the options for the init command
type InitOptions struct {
	Force bool
}

// NewInitOptions creates a new InitOptions
func NewInitOptions() *InitOptions {
	return &InitOptions{}
}
