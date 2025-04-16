package config

// TestOptions is the options for the build command
type TestOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// Cleanup is the cleanup flag
	Cleanup bool
	// Logs is the logs flagj
	Logs bool
	// Timeout is the timeout flag
	Timeout int
}

// NewTestOptions creates a new Apply
func NewTestOptions() *TestOptions {
	return &TestOptions{}
}
