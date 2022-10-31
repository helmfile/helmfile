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

// BuildImpl is impl for applyOptions
type BuildImpl struct {
	*GlobalImpl
	*BuildOptions
}

// NewBuildImpl creates a new BuildImpl
func NewBuildImpl(g *GlobalImpl, b *BuildOptions) *BuildImpl {
	return &BuildImpl{
		GlobalImpl:   g,
		BuildOptions: b,
	}
}

// EmbedValues returns the embed values.
func (b *BuildImpl) EmbedValues() bool {
	return b.BuildOptions.EmbedValues
}
