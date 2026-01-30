package config

type UnittestOptions struct {
	Values                 []string
	FailFast               bool
	Color                  bool
	DebugPlugin            bool
	UnittestArgs           []string
	Concurrency            int
	SkipNeeds              bool
	IncludeNeeds           bool
	IncludeTransitiveNeeds bool
}

func NewUnittestOptions() *UnittestOptions {
	return &UnittestOptions{}
}

type UnittestImpl struct {
	*GlobalImpl
	*UnittestOptions
}

func NewUnittestImpl(g *GlobalImpl, t *UnittestOptions) *UnittestImpl {
	return &UnittestImpl{
		GlobalImpl:      g,
		UnittestOptions: t,
	}
}

func (t *UnittestImpl) Values() []string {
	return t.UnittestOptions.Values
}

func (t *UnittestImpl) Concurrency() int {
	return t.UnittestOptions.Concurrency
}

func (t *UnittestImpl) FailFast() bool {
	return t.UnittestOptions.FailFast
}

func (t *UnittestImpl) Color() bool {
	return t.UnittestOptions.Color
}

func (t *UnittestImpl) DebugPlugin() bool {
	return t.UnittestOptions.DebugPlugin
}

func (t *UnittestImpl) UnittestArgs() []string {
	return t.UnittestOptions.UnittestArgs
}

func (t *UnittestImpl) Args() string {
	return ""
}

func (t *UnittestImpl) SkipNeeds() bool {
	return t.UnittestOptions.SkipNeeds
}

func (t *UnittestImpl) IncludeNeeds() bool {
	return t.UnittestOptions.IncludeNeeds
}

func (t *UnittestImpl) IncludeTransitiveNeeds() bool {
	return t.UnittestOptions.IncludeTransitiveNeeds
}

func (t *UnittestImpl) SkipDeps() bool {
	return false
}

func (t *UnittestImpl) SkipRefresh() bool {
	return false
}

func (t *UnittestImpl) SkipCleanup() bool {
	return false
}

func (t *UnittestImpl) EnforceNeedsAreInstalled() bool {
	return false
}
