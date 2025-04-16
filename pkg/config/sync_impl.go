package config

// SyncImpl is impl for SyncOptions
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

// SkipCRDs returns the skip crds
func (t *SyncImpl) SkipCRDs() bool {
	return t.SyncOptions.SkipCRDsFlag.Value()
}

// IncludeCRDs returns the include crds
func (t *SyncImpl) IncludeCRDs() bool {
	return t.SyncOptions.IncludeCRDsFlag.Value()
}

// ShouldIncludeCRDs determines if CRDs should be included in the operation.
func (t *SyncImpl) ShouldIncludeCRDs() bool {
	return ShouldIncludeCRDs(t.IncludeCRDsFlag, t.SkipCRDsFlag)
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
