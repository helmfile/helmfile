package config

// SyncOptions is the options for the build command
type SyncOptions struct {
	// Args is the args to pass to helm template
	Args string
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
	// SkipDeps is the skip deps flag
	SkipDeps bool
	// SkipCrds is the skip crds flag
	SkipCRDs bool
	// Wait is the wait flag
	Wait bool
	// WaitForJobs is the wait for jobs flag
	WaitForJobs bool
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

// Args returns the args
func (t *SyncImpl) Args() string {
	return t.SyncOptions.Args
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

// SkipDeps returns the skip deps
func (t *SyncImpl) SkipDeps() bool {
	return t.SyncOptions.SkipDeps
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

// WaitForJobs returns the wait for jobs
func (t *SyncImpl) WaitForJobs() bool {
	return t.SyncOptions.WaitForJobs
}
