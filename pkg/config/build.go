package config

// BuildOptions is the options for the build command
type BuildOptions struct {
	// EmbedValues is true if the values should be embedded
	EmbedValues bool
}

// NewBuildOptions creates a new Apply
func NewBuildOptions() *BuildOptions {
	return &BuildOptions{}
}
