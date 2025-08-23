package config

// ShowDAGOptions is the options for the build command
type ShowDAGOptions struct {
}

// NewShowDAGOptions creates a new ShowDAGOptions
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
