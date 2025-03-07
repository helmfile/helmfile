package config

// ApplyOptoons is the options for the apply command
type ApplyOptions struct {
	// Set is a list of key value pairs to be merged into the command
	Set []string
	// Values is a list of value files to be merged into the command
	Values []string
	// Concurrency is the maximum number of concurrent helm processes to run
	Concurrency int
	// Validate is validate your manifests against the Kubernetes cluster you are currently pointing at. Note that this requires access to a Kubernetes cluster to obtain information necessary for validating, like the list of available API versions
	Validate bool
	// Context is the number of lines of context to show around changes
	Context int
	// Output is the output format for the diff plugin
	Output string
	// DetailedExitcode is true if the exit code should be 2 instead of 0 if there were changes detected and the changes were synced successfully
	DetailedExitcode bool
	// StripTrailingCR is true if trailing carriage returns should be stripped during diffing
	StripTrailingCR bool
	// TODO: Remove this function once Helmfile v0.x
	// DEPRECATED: Use skip-cleanup instead
	RetainValuesFiles bool

	// SkipCleanup is true if the cleanup of temporary values files should be skipped
	SkipCleanup bool
	// SkipCRDs is true if the CRDs should be skipped
	SkipCRDs bool
	// SkipNeeds is true if the needs should be skipped
	SkipNeeds bool
	// IncludeNeeds is true if the needs should be included
	IncludeNeeds bool
	// IncludeTransitiveNeeds is true if the transitive needs should be included
	IncludeTransitiveNeeds bool
	// SkipDiffOnInstall is true if the diff should be skipped on install
	SkipDiffOnInstall bool
	// DiffArgs is the list of arguments to pass to the helm-diff.
	DiffArgs string
	// IncludeTests is true if the tests should be included
	IncludeTests bool
	// Suppress is true if the output should be suppressed
	Suppress []string
	// SuppressSecrets is true if the secrets should be suppressed
	SuppressSecrets bool
	// ShowSecrets is true if the secrets should be shown
	ShowSecrets bool
	// NoHooks skips checking for hooks
	NoHooks bool
	// SuppressDiff is true if the diff should be suppressed
	SuppressDiff bool
	// Wait is true if the helm command should wait for the release to be deployed
	Wait bool
	// WaitRetries is the number of times to retry waiting for the release to be deployed
	WaitRetries int
	// WaitForJobs is true if the helm command should wait for the jobs to be completed
	WaitForJobs bool
	// Propagate '--skip-schema-validation' to helmv3 template and helm install
	SkipSchemaValidation bool
	// ReuseValues is true if the helm command should reuse the values
	ReuseValues bool
	// ResetValues is true if helm command should reset values to charts' default
	ResetValues bool
	// Propagate '--post-renderer' to helmv3 template and helm install
	PostRenderer string
	// Propagate '--post-renderer-args' to helmv3 template and helm install
	PostRendererArgs []string
	// Cascade '--cascade' to helmv3 delete, available values: background, foreground, or orphan, default: background
	Cascade string
	// SuppressOutputLineRegex is a list of regexes to suppress output lines
	SuppressOutputLineRegex []string
	// SyncArgs is the list of arguments to pass to helm upgrade.
	SyncArgs string
	// HideNotes is the hide notes flag
	HideNotes bool

	// TakeOwnership is true if the ownership should be taken
	TakeOwnership bool
	// TemplateArgs is the list of arguments to pass to helm template
	TemplateArgs string
}

// NewApply creates a new Apply
func NewApplyOptions() *ApplyOptions {
	return &ApplyOptions{}
}

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
func (a *ApplyImpl) Set() []string {
	return a.ApplyOptions.Set
}

// Concurrency returns the concurrency.
func (a *ApplyImpl) Concurrency() int {
	return a.ApplyOptions.Concurrency
}

// Context returns the context.
func (a *ApplyImpl) Context() int {
	return a.ApplyOptions.Context
}

// DetailedExitcode returns the detailed exitcode.
func (a *ApplyImpl) DetailedExitcode() bool {
	return a.ApplyOptions.DetailedExitcode
}

// StripTrailingCR is true if trailing carriage returns should be stripped during diffing
func (a *ApplyImpl) StripTrailingCR() bool {
	return a.ApplyOptions.StripTrailingCR
}

// DiffOutput returns the diff output.
func (a *ApplyImpl) DiffOutput() string {
	return a.ApplyOptions.Output
}

// IncludeNeeds returns the include needs.
func (a *ApplyImpl) IncludeNeeds() bool {
	return a.ApplyOptions.IncludeNeeds || a.IncludeTransitiveNeeds()
}

// IncludeTests returns the include tests.
func (a *ApplyImpl) IncludeTests() bool {
	return a.ApplyOptions.IncludeTests
}

// IncludeTransitiveNeeds returns the include transitive needs.
func (a *ApplyImpl) IncludeTransitiveNeeds() bool {
	return a.ApplyOptions.IncludeTransitiveNeeds
}

// TODO: Remove this function once Helmfile v0.x
// RetainValuesFiles returns the retain values files.
func (a *ApplyImpl) RetainValuesFiles() bool {
	return a.ApplyOptions.RetainValuesFiles
}

// ShowSecrets returns the show secrets.
func (a *ApplyImpl) ShowSecrets() bool {
	return a.ApplyOptions.ShowSecrets
}

// NoHooks skips hooks.
func (a *ApplyImpl) NoHooks() bool {
	return a.ApplyOptions.NoHooks
}

// SkipCRDs returns the skip crds.
func (a *ApplyImpl) SkipCRDs() bool {
	return a.ApplyOptions.SkipCRDs
}

// SkipCleanup returns the skip cleanup.
func (a *ApplyImpl) SkipCleanup() bool {
	return a.ApplyOptions.SkipCleanup
}

// SkipDiffOnInstall returns the skip diff on install.
func (a *ApplyImpl) SkipDiffOnInstall() bool {
	return a.ApplyOptions.SkipDiffOnInstall
}

// DiffArgs is the list of arguments to pass to helm-diff.
func (a *ApplyImpl) DiffArgs() string {
	return a.ApplyOptions.DiffArgs
}

// SkipNeeds returns the skip needs.
func (a *ApplyImpl) SkipNeeds() bool {
	if !a.IncludeNeeds() {
		return a.ApplyOptions.SkipNeeds
	}
	return false
}

// Suppress returns the suppress.
func (a *ApplyImpl) Suppress() []string {
	return a.ApplyOptions.Suppress
}

// SuppressDiff returns the suppress diff.
func (a *ApplyImpl) SuppressDiff() bool {
	return a.ApplyOptions.SuppressDiff
}

// SuppressSecrets returns the suppress secrets.
func (a *ApplyImpl) SuppressSecrets() bool {
	return a.ApplyOptions.SuppressSecrets
}

// Validate returns the validate.
func (a *ApplyImpl) Validate() bool {
	return a.ApplyOptions.Validate
}

// Values returns the values.
func (a *ApplyImpl) Values() []string {
	return a.ApplyOptions.Values
}

// Wait returns the wait.
func (a *ApplyImpl) Wait() bool {
	return a.ApplyOptions.Wait
}

// WaitRetries returns the wait retries.
func (a *ApplyImpl) WaitRetries() int {
	return a.ApplyOptions.WaitRetries
}

// WaitForJobs returns the wait for jobs.
func (a *ApplyImpl) WaitForJobs() bool {
	return a.ApplyOptions.WaitForJobs
}

// ReuseValues returns the ReuseValues.
func (a *ApplyImpl) ReuseValues() bool {
	if !a.ResetValues() {
		return a.ApplyOptions.ReuseValues
	}
	return false
}

func (a *ApplyImpl) ResetValues() bool {
	return a.ApplyOptions.ResetValues
}

// PostRenderer returns the PostRenderer.
func (a *ApplyImpl) PostRenderer() string {
	return a.ApplyOptions.PostRenderer
}

// PostRendererArgs returns the PostRendererArgs.
func (a *ApplyImpl) PostRendererArgs() []string {
	return a.ApplyOptions.PostRendererArgs
}

// SkipSchemaValidation returns the SkipSchemaValidation.
func (a *ApplyImpl) SkipSchemaValidation() bool {
	return a.ApplyOptions.SkipSchemaValidation
}

// Cascade returns cascade flag
func (a *ApplyImpl) Cascade() string {
	return a.ApplyOptions.Cascade
}

// SuppressOutputLineRegex returns the SuppressOutputLineRegex.
func (a *ApplyImpl) SuppressOutputLineRegex() []string {
	return a.ApplyOptions.SuppressOutputLineRegex
}

// SyncArgs returns the SyncArgs.
func (a *ApplyImpl) SyncArgs() string {
	return a.ApplyOptions.SyncArgs
}

// HideNotes returns the HideNotes.
func (a *ApplyImpl) HideNotes() bool {
	return a.ApplyOptions.HideNotes
}

// TakeOwnership returns the TakeOwnership.
func (a *ApplyImpl) TakeOwnership() bool {
	return a.ApplyOptions.TakeOwnership
}

// TemplateArgs returns the TemplateArgs.
func (a *ApplyImpl) TemplateArgs() string {
	return a.ApplyOptions.TemplateArgs
}
