package config

// ApplyImpl is impl for applyOptions
type ApplyImpl struct {
	*GlobalImpl
	*ApplyOptions
}

// NewApplyImpl creates a new ApplyImpl
func NewApplyImpl(g *GlobalImpl, a *ApplyOptions) *ApplyImpl {
	return &ApplyImpl{
		GlobalImpl:   g,
		ApplyOptions: a,
	}
}

// Set returns the set.
func (t *ApplyImpl) Set() []string {
	return t.ApplyOptions.Set
}

// Concurrency returns the concurrency.
func (t *ApplyImpl) Concurrency() int {
	return t.ApplyOptions.Concurrency
}

// Context returns the context.
func (t *ApplyImpl) Context() int {
	return t.ApplyOptions.Context
}

// DetailedExitcode returns the detailed exitcode.
func (t *ApplyImpl) DetailedExitcode() bool {
	return t.ApplyOptions.DetailedExitcode
}

// StripTrailingCR is true if trailing carriage returns should be stripped during diffing
func (t *ApplyImpl) StripTrailingCR() bool {
	return t.ApplyOptions.StripTrailingCR
}

// DiffOutput returns the diff output.
func (t *ApplyImpl) DiffOutput() string {
	return t.Output
}

// IncludeNeeds returns the include needs.
func (t *ApplyImpl) IncludeNeeds() bool {
	return t.ApplyOptions.IncludeNeeds || t.IncludeTransitiveNeeds()
}

// IncludeTests returns the include tests.
func (t *ApplyImpl) IncludeTests() bool {
	return t.ApplyOptions.IncludeTests
}

// IncludeTransitiveNeeds returns the include transitive needs.
func (t *ApplyImpl) IncludeTransitiveNeeds() bool {
	return t.ApplyOptions.IncludeTransitiveNeeds
}

// ShowSecrets returns the show secrets.
func (t *ApplyImpl) ShowSecrets() bool {
	return t.ApplyOptions.ShowSecrets
}

// NoHooks skips hooks.
func (t *ApplyImpl) NoHooks() bool {
	return t.ApplyOptions.NoHooks
}

// SkipCRDs returns the skip CRDs.
func (t *ApplyImpl) SkipCRDs() bool {
	return t.ApplyOptions.SkipCRDsFlag.Value()
}

// IncludeCRDs returns the include CRDs.
func (t *ApplyImpl) IncludeCRDs() bool {
	return t.ApplyOptions.IncludeCRDsFlag.Value()
}

// ShouldIncludeCRDs determines if CRDs should be included in the operation.
func (t *ApplyImpl) ShouldIncludeCRDs() bool {
	return ShouldIncludeCRDs(t.IncludeCRDsFlag, t.SkipCRDsFlag)
}

// SkipCleanup returns the skip cleanup.
func (t *ApplyImpl) SkipCleanup() bool {
	return t.ApplyOptions.SkipCleanup
}

// SkipDiffOnInstall returns the skip diff on install.
func (t *ApplyImpl) SkipDiffOnInstall() bool {
	return t.ApplyOptions.SkipDiffOnInstall
}

// DiffArgs is the list of arguments to pass to helm-diff.
func (t *ApplyImpl) DiffArgs() string {
	return t.ApplyOptions.DiffArgs
}

// SkipNeeds returns the skip needs.
func (t *ApplyImpl) SkipNeeds() bool {
	if !t.IncludeNeeds() {
		return t.ApplyOptions.SkipNeeds
	}
	return false
}

// Suppress returns the suppress.
func (t *ApplyImpl) Suppress() []string {
	return t.ApplyOptions.Suppress
}

// SuppressDiff returns the suppress diff.
func (t *ApplyImpl) SuppressDiff() bool {
	return t.ApplyOptions.SuppressDiff
}

// SuppressSecrets returns the suppress secrets.
func (t *ApplyImpl) SuppressSecrets() bool {
	return t.ApplyOptions.SuppressSecrets
}

// Validate returns the validate.
func (t *ApplyImpl) Validate() bool {
	return t.ApplyOptions.Validate
}

// Values returns the values.
func (t *ApplyImpl) Values() []string {
	return t.ApplyOptions.Values
}

// Wait returns the wait.
func (t *ApplyImpl) Wait() bool {
	return t.ApplyOptions.Wait
}

// WaitRetries returns the wait retries.
func (t *ApplyImpl) WaitRetries() int {
	return t.ApplyOptions.WaitRetries
}

// WaitForJobs returns the wait for jobs.
func (t *ApplyImpl) WaitForJobs() bool {
	return t.ApplyOptions.WaitForJobs
}

// ReuseValues returns the ReuseValues.
func (t *ApplyImpl) ReuseValues() bool {
	if !t.ResetValues() {
		return t.ApplyOptions.ReuseValues
	}
	return false
}

func (t *ApplyImpl) ResetValues() bool {
	return t.ApplyOptions.ResetValues
}

// PostRenderer returns the PostRenderer.
func (t *ApplyImpl) PostRenderer() string {
	return t.ApplyOptions.PostRenderer
}

// PostRendererArgs returns the PostRendererArgs.
func (t *ApplyImpl) PostRendererArgs() []string {
	return t.ApplyOptions.PostRendererArgs
}

// SkipSchemaValidation returns the SkipSchemaValidation.
func (t *ApplyImpl) SkipSchemaValidation() bool {
	return t.ApplyOptions.SkipSchemaValidation
}

// Cascade returns cascade flag
func (t *ApplyImpl) Cascade() string {
	return t.ApplyOptions.Cascade
}

// SuppressOutputLineRegex returns the SuppressOutputLineRegex.
func (t *ApplyImpl) SuppressOutputLineRegex() []string {
	return t.ApplyOptions.SuppressOutputLineRegex
}

// SyncArgs returns the SyncArgs.
func (t *ApplyImpl) SyncArgs() string {
	return t.ApplyOptions.SyncArgs
}

// HideNotes returns the HideNotes.
func (t *ApplyImpl) HideNotes() bool {
	return t.ApplyOptions.HideNotes
}

// TakeOwnership returns the TakeOwnership.
func (t *ApplyImpl) TakeOwnership() bool {
	return t.ApplyOptions.TakeOwnership
}

func (t *ApplyImpl) SyncReleaseLabels() bool {
	return t.ApplyOptions.SyncReleaseLabels
}
