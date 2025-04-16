package config

// BuildImpl is impl for BuildOptions
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
