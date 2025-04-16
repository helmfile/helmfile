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
