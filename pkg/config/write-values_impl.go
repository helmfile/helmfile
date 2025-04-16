package config

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
