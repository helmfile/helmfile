package config

// WriteValuesOptions is the options for the build command
type WriteValuesOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// SkipDeps is the skip deps flag
	SkipDeps bool
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

// WriteValuesImpl is impl for applyOptions
type WriteValuesImpl struct {
	*GlobalImpl
	*WriteValuesOptions
}

// NewWriteValuesImpl creates a new WriteValuesImpl
func NewWriteValuesImpl(g *GlobalImpl, b *WriteValuesOptions) *WriteValuesImpl {
	return &WriteValuesImpl{
		GlobalImpl:         g,
		WriteValuesOptions: b,
	}
}

// Concurrency returns the concurrency
func (c *WriteValuesImpl) Concurrency() int {
	return c.WriteValuesOptions.Concurrency
}

// SkipDeps returns the skip deps
func (c *WriteValuesImpl) SkipDeps() bool {
	return c.GlobalOptions.SkipDeps || c.WriteValuesOptions.SkipDeps
}

// Set returns the Set
func (c *WriteValuesImpl) Set() []string {
	return c.WriteValuesOptions.Set
}

// Values returns the Values
func (c *WriteValuesImpl) Values() []string {
	return c.WriteValuesOptions.Values
}

// SkipCleanUp returns the skip clean up
func (c *WriteValuesImpl) SkipCleanup() bool {
	return false
}

// IncludeTransitiveNeeds returns the include transitive needs
func (c *WriteValuesImpl) IncludeTransitiveNeeds() bool {
	return false
}

// OutputFileTemplate returns the output file template
func (c *WriteValuesImpl) OutputFileTemplate() string {
	return c.WriteValuesOptions.OutputFileTemplate
}
