package config

// SyncOptions is the options for the build command
type SyncOptions struct {
	// Set is the set flag
	Set []string
	// Values is the values flag
	Values []string
	// Concurrency is the concurrency flag
	Concurrency int
	// Validate is the validate flag
	Validate bool
	// IncludeCRDs is the include crds flag
	SkipNeeds bool
	// IncludeNeeds is the include needs flag
	IncludeNeeds bool
	// IncludeTransitiveNeeds is the include transitive needs flag
	IncludeTransitiveNeeds bool
	// SkipCrds is the skip crds flag
	SkipCRDs bool
	// Wait is the wait flag
	Wait bool
	// WaitRetries is the wait retries flag
	WaitRetries int
	// WaitForJobs is the wait for jobs flag
	WaitForJobs bool
	// ReuseValues is true if the helm command should reuse the values
	ReuseValues bool
	// ResetValues is true if helm command should reset values to charts' default
	ResetValues bool
	// Propagate '--post-renderer' to helmv3 template and helm install
	PostRenderer string
	// Propagate '--post-renderer-args' to helmv3 template and helm install
	PostRendererArgs []string
	// Propagate '--skip-schema-validation' to helmv3 template and helm install
	SkipSchemaValidation bool
	// Cascade '--cascade' to helmv3 delete, available values: background, foreground, or orphan, default: background
	Cascade string
	// SyncArgs is the list of arguments to pass to the helm upgrade command.
	SyncArgs string
	// HideNotes is the hide notes flag
	HideNotes bool
	// TakeOwnership is the take ownership flag
	TakeOwnership bool
	// SyncReleaseLabels is the sync release labels flag
	SyncReleaseLabels bool
}

// NewSyncOptions creates a new Apply
func NewSyncOptions() *SyncOptions {
	return &SyncOptions{}
}

// SyncImpl is impl for applyOptions
type SyncImpl struct {
	*GlobalImpl
	*SyncOptions
}

// NewSyncImpl creates a new SyncImpl
func NewSyncImpl(g *GlobalImpl, t *SyncOptions) *SyncImpl {
	return &SyncImpl{
		GlobalImpl:  g,
		SyncOptions: t,
	}
}

// Concurrency returns the concurrency
func (t *SyncImpl) Concurrency() int {
	return t.SyncOptions.Concurrency
}

// IncludeNeeds returns the include needs
func (t *SyncImpl) IncludeNeeds() bool {
	return t.SyncOptions.IncludeNeeds || t.IncludeTransitiveNeeds()
}

// IncludeTransitiveNeeds returns the include transitive needs
func (t *SyncImpl) IncludeTransitiveNeeds() bool {
	return t.SyncOptions.IncludeTransitiveNeeds
}

// Set returns the Set
func (t *SyncImpl) Set() []string {
	return t.SyncOptions.Set
}

// SkipNeeds returns the skip needs
func (t *SyncImpl) SkipNeeds() bool {
	if !t.IncludeNeeds() {
		return t.SyncOptions.SkipNeeds
	}

	return false
}

// Validate returns the validate
func (t *SyncImpl) Validate() bool {
	return t.SyncOptions.Validate
}

// Values returns the values
func (t *SyncImpl) Values() []string {
	return t.SyncOptions.Values
}

// SkipCRDS returns the skip crds
func (t *SyncImpl) SkipCRDs() bool {
	return t.SyncOptions.SkipCRDs
}

// Wait returns the wait
func (t *SyncImpl) Wait() bool {
	return t.SyncOptions.Wait
}

// WaitRetries returns the wait retries
func (t *SyncImpl) WaitRetries() int {
	return t.SyncOptions.WaitRetries
}

// WaitForJobs returns the wait for jobs
func (t *SyncImpl) WaitForJobs() bool {
	return t.SyncOptions.WaitForJobs
}

// ReuseValues returns the ReuseValues.
func (t *SyncImpl) ReuseValues() bool {
	if !t.ResetValues() {
		return t.SyncOptions.ReuseValues
	}
	return false
}
func (t *SyncImpl) ResetValues() bool {
	return t.SyncOptions.ResetValues
}

// PostRenderer returns the PostRenderer.
func (t *SyncImpl) PostRenderer() string {
	return t.SyncOptions.PostRenderer
}

// PostRendererArgs returns the PostRendererArgs.
func (t *SyncImpl) PostRendererArgs() []string {
	return t.SyncOptions.PostRendererArgs
}

// SkipSchemaValidation returns the SkipSchemaValidation.
func (t *SyncImpl) SkipSchemaValidation() bool {
	return t.SyncOptions.SkipSchemaValidation
}

// Cascade returns cascade flag
func (t *SyncImpl) Cascade() string {
	return t.SyncOptions.Cascade
}

// SyncArgs returns the sync args
func (t *SyncImpl) SyncArgs() string {
	return t.SyncOptions.SyncArgs
}

// HideNotes returns the hide notes
func (t *SyncImpl) HideNotes() bool {
	return t.SyncOptions.HideNotes
}

// TakeOwnership returns the take ownership
func (t *SyncImpl) TakeOwnership() bool {
	return t.SyncOptions.TakeOwnership
}

func (t *SyncImpl) SyncReleaseLabels() bool {
	return t.SyncOptions.SyncReleaseLabels
}
