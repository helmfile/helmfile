package config

// PrintEnvOptions is the options for the print-env command
type PrintEnvOptions struct {
	// Output is the output format (yaml or json)
	Output string
}

// NewPrintEnvOptions creates a new PrintEnvOptions
func NewPrintEnvOptions() *PrintEnvOptions {
	return &PrintEnvOptions{
		Output: "yaml", // default to yaml
	}
}

// PrintEnvImpl is impl for PrintEnvOptions
type PrintEnvImpl struct {
	*GlobalImpl
	*PrintEnvOptions
}

// NewPrintEnvImpl creates a new PrintEnvImpl
func NewPrintEnvImpl(g *GlobalImpl, p *PrintEnvOptions) *PrintEnvImpl {
	return &PrintEnvImpl{
		GlobalImpl:      g,
		PrintEnvOptions: p,
	}
}

// Output returns the output format
func (c *PrintEnvImpl) Output() string {
	return c.PrintEnvOptions.Output
}
