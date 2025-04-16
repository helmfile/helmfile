package config

// DepsOptions is the options for the build command
type DepsOptions struct {
	// SkipRepos is the skip repos flag
	SkipRepos bool
	// Concurrency is the maximum number of concurrent helm processes to run
	Concurrency int
}

// NewDepsOptions creates a new Apply
func NewDepsOptions() *DepsOptions {
	return &DepsOptions{}
}
