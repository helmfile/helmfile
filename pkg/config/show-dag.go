package config

// ShowDAGOptions is the options for the build command
type ShowDAGOptions struct {
	// EmbedValues is true if the values should be embedded
	EmbedValues bool
}

// NewShowDAGOptions creates a new Apply
func NewShowDAGOptions() *ShowDAGOptions {
	return &ShowDAGOptions{}
}

// ShowDAGImpl is impl for applyOptions
type ShowDAGImpl struct {
	*GlobalImpl
	*ShowDAGOptions
}

// NewShowDAGImpl creates a new ShowDAGImpl
func NewShowDAGImpl(g *GlobalImpl, b *ShowDAGOptions) *ShowDAGImpl {
	return &ShowDAGImpl{
		GlobalImpl:     g,
		ShowDAGOptions: b,
	}
}

// EmbedValues returns the embed values.
func (b *ShowDAGImpl) EmbedValues() bool {
	return b.ShowDAGOptions.EmbedValues
}
