package config

// FetchImpl is impl for FechtOptions
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
