package config

// LintOptions is the options for the build command
type LintOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// Set is the set flags to pass to helm lint
	Set []string
	// Values is the values flags to pass to helm lint
	Values []string
	// SkipNeeds is the skip needs flag
	SkipNeeds bool
	// IncludeNeeds is the include needs flag
	IncludeNeeds bool
	// IncludeTransitiveNeeds is the include transitive needs flag
	IncludeTransitiveNeeds bool
	// SkipDeps is the skip deps flag
}

// NewLintOptions creates a new Apply
func NewLintOptions() *LintOptions {
	return &LintOptions{}
}
