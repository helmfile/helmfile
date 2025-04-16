package config

// DiffImpl is impl for DiffOptions
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

// Concurrency returns the concurrency
func (t *DiffImpl) Concurrency() int {
	return t.DiffOptions.Concurrency
}

// IncludeNeeds returns the include needs
func (t *DiffImpl) IncludeNeeds() bool {
	return t.DiffOptions.IncludeNeeds || t.IncludeTransitiveNeeds()
}

// IncludeTransitiveNeeds returns the include transitive needs
func (t *DiffImpl) IncludeTransitiveNeeds() bool {
	return t.DiffOptions.IncludeTransitiveNeeds
}

// Set returns the Set
func (t *DiffImpl) Set() []string {
	return t.DiffOptions.Set
}

// SkipNeeds returns the skip needs
func (t *DiffImpl) SkipNeeds() bool {
	if !t.IncludeNeeds() {
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
	return t.DiffOptions.Context
}

// DetailedExitCode returns the detailed exit code
func (t *DiffImpl) DetailedExitcode() bool {
	return t.DiffOptions.DetailedExitcode
}

// StripTrailingCR is true if trailing carriage returns should be stripped during diffing
func (a *DiffImpl) StripTrailingCR() bool {
	return a.DiffOptions.StripTrailingCR
}

// Output returns the output
func (t *DiffImpl) DiffOutput() string {
	return t.Output
}

// IncludeTests returns the include tests
func (t *DiffImpl) IncludeTests() bool {
	return t.DiffOptions.IncludeTests
}

// ShowSecrets returns the show secrets
func (t *DiffImpl) ShowSecrets() bool {
	return t.DiffOptions.ShowSecrets
}

// NoHooks skips hooks.
func (t *DiffImpl) NoHooks() bool {
	return t.DiffOptions.NoHooks
}

// SkipCRDs returns the skip crds
func (t *DiffImpl) SkipCRDs() bool {
	return t.DiffOptions.SkipCRDsFlag.Value()
}

// IncludeCRDs returns the include crds
func (t *DiffImpl) IncludeCRDs() bool {
	return t.DiffOptions.IncludeCRDsFlag.Value()
}

// ShouldIncludeCRDs determines if CRDs should be included in the operation.
func (t *DiffImpl) ShouldIncludeCRDs() bool {
	return ShouldIncludeCRDs(t.IncludeCRDsFlag, t.SkipCRDsFlag)
}

// SkipDiffOnInstall returns the skip diff on install
func (t *DiffImpl) SkipDiffOnInstall() bool {
	return t.DiffOptions.SkipDiffOnInstall
}

// DiffArgs returns the list of arguments to pass to helm-diff.
func (t *DiffImpl) DiffArgs() string {
	return t.DiffOptions.DiffArgs
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

// ReuseValues returns the ReuseValues.
func (t *DiffImpl) ReuseValues() bool {
	if !t.ResetValues() {
		return t.DiffOptions.ReuseValues
	}
	return false
}

func (t *DiffImpl) ResetValues() bool {
	return t.DiffOptions.ResetValues
}

// PostRenderer returns the PostRenderer.
func (t *DiffImpl) PostRenderer() string {
	return t.DiffOptions.PostRenderer
}

// PostRendererArgs returns the PostRendererArgs.
func (t *DiffImpl) PostRendererArgs() []string {
	return t.DiffOptions.PostRendererArgs
}

// SuppressOutputLineRegex returns the SuppressOutputLineRegex.
func (t *DiffImpl) SuppressOutputLineRegex() []string {
	return t.DiffOptions.SuppressOutputLineRegex
}

func (t *DiffImpl) SkipSchemaValidation() bool {
	return t.DiffOptions.SkipSchemaValidation
}
