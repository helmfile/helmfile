package config

// FetchOptions is the options for the fetch command
type FetchOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// OutputDir is the output directory
	OutputDir string
	// OutputDirTemplate is the go template to generate the path of output directory
	OutputDirTemplate string
	// WriteOutput writes a helmfile.yaml with chart references updated to point to downloaded local chart paths
	WriteOutput bool
}

// NewFetchOptions creates a new FetchOptions
func NewFetchOptions() *FetchOptions {
	return &FetchOptions{}
}

// FetchImpl is impl for fetchOptions
type FetchImpl struct {
	*GlobalImpl
	*FetchOptions
}

// NewFetchImpl creates a new FetchImpl
func NewFetchImpl(g *GlobalImpl, b *FetchOptions) *FetchImpl {
	return &FetchImpl{
		GlobalImpl:   g,
		FetchOptions: b,
	}
}

// Concurrency returns the concurrency
func (c *FetchImpl) Concurrency() int {
	return c.FetchOptions.Concurrency
}

// OutputDir returns the args
func (c *FetchImpl) OutputDir() string {
	return c.FetchOptions.OutputDir
}

// OutputDirTemplate returns the go template to generate the path of output directory
func (c *FetchImpl) OutputDirTemplate() string {
	return c.FetchOptions.OutputDirTemplate
}

// WriteOutput returns whether to write a modified helmfile.yaml with local chart paths
func (c *FetchImpl) WriteOutput() bool {
	return c.FetchOptions.WriteOutput
}
