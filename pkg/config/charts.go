// TODO: Remove this function once Helmfile v0.x
package config

// ChartsOptions is the options for the build command
type ChartsOptions struct {
	// Set is the additional values to be merged into the command
	Set []string
	// Values is the additional value files to be merged into the command
	Values []string
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
}

// NewChartsOptions creates a new Apply
func NewChartsOptions() *ChartsOptions {
	return &ChartsOptions{}
}

// ChartsImpl is impl for applyOptions
type ChartsImpl struct {
	*GlobalImpl
	*ChartsOptions
}

// NewChartsImpl creates a new ChartsImpl
func NewChartsImpl(g *GlobalImpl, b *ChartsOptions) *ChartsImpl {
	return &ChartsImpl{
		GlobalImpl:    g,
		ChartsOptions: b,
	}
}

// Concurrency returns the concurrency
func (c *ChartsImpl) Concurrency() int {
	return c.ChartsOptions.Concurrency
}

// IncludeTransitiveNeeds returns the includeTransitiveNeeds
func (c *ChartsImpl) IncludeTransitiveNeeds() bool {
	return false
}

// Values returns the values
func (c *ChartsImpl) Values() []string {
	return c.ChartsOptions.Values
}
