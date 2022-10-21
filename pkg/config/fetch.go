package config

// FetchOptions is the options for the build command
type FetchOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// SkipDeps is the skip deps flag
	SkipDeps bool
	// OutputDir is the output directory
	OutputDir         string
	OutputDirTemplate string
}

// NewFetchOptions creates a new Apply
func NewFetchOptions() *FetchOptions {
	return &FetchOptions{}
}

// FetchImpl is impl for applyOptions
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

// SkipDeps returns the skip deps
func (c *FetchImpl) SkipDeps() bool {
	return c.FetchOptions.SkipDeps
}

// OutputDir returns the args
func (c *FetchImpl) OutputDir() string {
	return c.FetchOptions.OutputDir
}

func (c *FetchImpl) OutputDirTemplate() string {
	return c.FetchOptions.OutputDirTemplate
}
