package config

// WriteValuesOptions is the options for the build command
type WriteValuesOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// Set is the set flags to pass to helm write values
	Set []string
	// Values is the values flags to pass to helm write values
	Values []string
	// OutputFileTemplate is the output file template
	OutputFileTemplate string
}

// NewWriteValuesOptions creates a new Apply
func NewWriteValuesOptions() *WriteValuesOptions {
	return &WriteValuesOptions{}
}
