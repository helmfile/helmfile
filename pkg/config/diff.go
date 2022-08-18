package config

// DiffOptions is the options for the build command
type DiffOptions struct {
	// Args is the args to pass to helm template
	Args string
	// Set is the set flag
	Set []string
	// Values is the values flag
	Values []string
	// SkipDeps is the skip deps flag
	SkipDeps bool
	// DetailedExitcode is the detailed exit code
	DetailedExitcode bool
	// IncludeTests is the include tests flag
	IncludeTests bool
	// SkipNeeds is the include crds flag
	SkipNeeds bool
	// IncludeNeeds is the include needs flag
	IncludeNeeds bool
	// IncludeTransitiveNeeds is the include transitive needs flag
	IncludeTransitiveNeeds bool
	// SkipDiffOnInstall is the skip diff on install flag
	SkipDiffOnInstall bool
	// ShowSecrets is the show secrets flag
	ShowSecrets bool
	// Suppress is the suppress flag
	Suppress []string
	// SuppressSecrets is the suppress secrets flag
	SuppressSecrets bool
	// Concurrency is the concurrency flag
	Concurrency int
	// Validate is the validate flag
	Validate bool
	// Context is the context flag
	Context int
	// Output is output flag
	Output string
}

// NewDiffOptions creates a new Apply
func NewDiffOptions() *DiffOptions {
	return &DiffOptions{}
}

// DiffImpl is impl for applyOptions
type DiffImpl struct {
	*GlobalImpl
	*DiffOptions
}

// NewDiffImpl creates a new DiffImpl
func NewDiffImpl(g *GlobalImpl, t *DiffOptions) *DiffImpl {
	return &DiffImpl{
		GlobalImpl:  g,
		DiffOptions: t,
	}
}

// Args returns the args
func (t *DiffImpl) Args() string {
	return t.DiffOptions.Args
}

// Concurrency returns the concurrency
func (t *DiffImpl) Concurrency() int {
	return t.DiffOptions.Concurrency
}

// IncludeNeeds returns the include needs
func (t *DiffImpl) IncludeNeeds() bool {
	return t.DiffOptions.IncludeNeeds || t.DiffOptions.IncludeTransitiveNeeds
}

// IncludeTransitiveNeeds returns the include transitive needs
func (t *DiffImpl) IncludeTransitiveNeeds() bool {
	return t.DiffOptions.IncludeTransitiveNeeds
}

// Set returns the Set
func (t *DiffImpl) Set() []string {
	return t.DiffOptions.Set
}

// SkipDeps returns the skip deps
func (t *DiffImpl) SkipDeps() bool {
	return t.DiffOptions.SkipDeps
}

// SkipNeeds returns the skip needs
func (t *DiffImpl) SkipNeeds() bool {
	if !t.DiffOptions.IncludeNeeds {
		return t.DiffOptions.SkipNeeds
	}

	return false
}

// Validate returns the validate
func (t *DiffImpl) Validate() bool {
	return t.DiffOptions.Validate
}

// Values returns the values
func (t *DiffImpl) Values() []string {
	return t.DiffOptions.Values
}

// Context returns the context
func (t *DiffImpl) Context() int {
	return 0
}

// DetailedExitCode returns the detailed exit code
func (t *DiffImpl) DetailedExitcode() bool {
	return t.DiffOptions.DetailedExitcode
}

// Output returns the output
func (t *DiffImpl) DiffOutput() string {
	return t.DiffOptions.Output
}

// IncludeTests returns the include tests
func (t *DiffImpl) IncludeTests() bool {
	return t.DiffOptions.IncludeTests
}

// ShowSecrets returns the show secrets
func (t *DiffImpl) ShowSecrets() bool {
	return t.DiffOptions.ShowSecrets
}

// ShowCRDs returns the show crds
func (t *DiffImpl) SkipCRDs() bool {
	return false
}

// SkipDiffOnInstall returns the skip diff on install
func (t *DiffImpl) SkipDiffOnInstall() bool {
	return t.DiffOptions.SkipDiffOnInstall
}

// Suppress returns the suppress
func (t *DiffImpl) Suppress() []string {
	return t.DiffOptions.Suppress
}

// SuppressDiff returns the suppress diff
func (t *DiffImpl) SuppressDiff() bool {
	return false
}

// SuppressSecrets returns the suppress secrets
func (t *DiffImpl) SuppressSecrets() bool {
	return t.DiffOptions.SuppressSecrets
}
