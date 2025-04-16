package config

// FetchOptions is the options for the build command
type FetchOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// OutputDir is the output directory
	OutputDir string
	// OutputDirTemplate is the go template to generate the path of output directory
	OutputDirTemplate string
}

// NewFetchOptions creates a new Apply
func NewFetchOptions() *FetchOptions {
	return &FetchOptions{}
}
