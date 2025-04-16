package config

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
