package config

import "fmt"

// PrintEnvOptions is the options for the print-env command
type PrintEnvOptions struct {
	// OutputFormat is the output format (yaml or json)
	OutputFormat string
}

// NewPrintEnvOptions creates a new PrintEnvOptions
func NewPrintEnvOptions() *PrintEnvOptions {
	return &PrintEnvOptions{}
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
	return c.OutputFormat
}

// ValidateConfig validates the print-env configuration
func (c *PrintEnvImpl) ValidateConfig() error {
	if c.OutputFormat != "" && c.OutputFormat != "yaml" && c.OutputFormat != "json" {
		return fmt.Errorf("invalid output format %q: must be 'yaml' or 'json'", c.OutputFormat)
	}
	return nil
}
