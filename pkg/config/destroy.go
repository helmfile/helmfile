package config

// DestroyOptions is the options for the build command
type DestroyOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// SkipCharts makes Destroy skip `withPreparedCharts`
	SkipCharts bool
	// Cascade '--cascade' to helmv3 delete, available values: background, foreground, or orphan, default: background
	Cascade string
	// Wait '--wait' if set, will wait until all the resources are destroyed before returning. It will wait for as long as --timeout
	DeleteWait bool
	// Timeout '--timeout', to wait for helm operation (default 5m0s)
	DeleteTimeout int
}

// NewDestroyOptions creates a new Apply
func NewDestroyOptions() *DestroyOptions {
	return &DestroyOptions{}
}
