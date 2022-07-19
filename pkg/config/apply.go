package config

import "fmt"

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
	// Args is the arguments to pass to helm exec
	Args string
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
	// IncludeTests is true if the tests should be included
	IncludeTests bool
	// Suppress is true if the output should be suppressed
	Suppress []string
	// SuppressSecrets is true if the secrets should be suppressed
	SuppressSecrets bool
	// SuppressDiff is true if the diff should be suppressed
	ShowSecrets bool
	// SkipDeps is true if the running "helm repo update" and "helm dependency build" should be skipped
	SuppressDiff bool
	// ShowSecrets is true if the secrets should be shown
	SkipDeps bool
	// Wait is true if the helm command should wait for the release to be deployed
	Wait bool
	// WaitForJobs is true if the helm command should wait for the jobs to be completed
	WaitForJobs bool
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

// Args returns the args.
func (a *ApplyImpl) Args() string {
	args := a.ApplyOptions.Args
	enableHelmDebug := a.GlobalImpl.Debug

	if enableHelmDebug {
		args = fmt.Sprintf("%s %s", args, "--debug")
	}

	return args
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

// DiffOutput returns the diff output.
func (a *ApplyImpl) DiffOutput() string {
	return a.ApplyOptions.Output
}

// IncludeNeeds returns the include needs.
func (a *ApplyImpl) IncludeNeeds() bool {
	return a.ApplyOptions.IncludeNeeds || a.ApplyOptions.IncludeTransitiveNeeds
}

// IncludeTests returns the include tests.
func (a *ApplyImpl) IncludeTests() bool {
	return a.ApplyOptions.IncludeTests
}

// IncludeTransitiveNeeds returns the include transitive needs.
func (a *ApplyImpl) IncludeTransitiveNeeds() bool {
	return a.ApplyOptions.IncludeTransitiveNeeds
}

// RetainValuesFiles returns the retain values files.
func (a *ApplyImpl) RetainValuesFiles() bool {
	return a.ApplyOptions.RetainValuesFiles
}

// ShowSecrets returns the show secrets.
func (a *ApplyImpl) ShowSecrets() bool {
	return a.ApplyOptions.ShowSecrets
}

// SkipCRDs returns the skip crds.
func (a *ApplyImpl) SkipCRDs() bool {
	return a.ApplyOptions.SkipCRDs
}

// SkipCleanup returns the skip cleanup.
func (a *ApplyImpl) SkipCleanup() bool {
	return a.ApplyOptions.SkipCleanup
}

// SkipDeps returns the skip deps.
func (a *ApplyImpl) SkipDeps() bool {
	return a.ApplyOptions.SkipDeps
}

// SkipDiffOnInstall returns the skip diff on install.
func (a *ApplyImpl) SkipDiffOnInstall() bool {
	return a.ApplyOptions.SkipDiffOnInstall
}

// SkipNeeds returns the skip needs.
func (a *ApplyImpl) SkipNeeds() bool {
	if !a.ApplyOptions.IncludeNeeds {
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

// WaitForJobs returns the wait for jobs.
func (a *ApplyImpl) WaitForJobs() bool {
	return a.ApplyOptions.WaitForJobs
}
