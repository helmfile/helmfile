package app

import "go.uber.org/zap"

type ConfigProvider interface {
	Args() string
	HelmBinary() string
	KustomizeBinary() string
	EnableLiveOutput() bool
	StripArgsValuesOnExitError() bool
	DisableForceUpdate() bool
	SkipDeps() bool
	SkipRefresh() bool

	FileOrDir() string
	KubeContext() string
	Namespace() string
	Chart() string
	Selectors() []string
	StateValuesSet() map[string]any
	StateValuesFiles() []string
	Kubeconfig() string
	Env() string

	loggingConfig
}

// TODO: Remove this function once Helmfile v0.x
type DeprecatedChartsConfigProvider interface {
	Values() []string

	concurrencyConfig
	loggingConfig
	IncludeTransitiveNeeds() bool
}

type DepsConfigProvider interface {
	Args() string
	SkipRepos() bool
	IncludeTransitiveNeeds() bool

	concurrencyConfig
}

type ReposConfigProvider interface {
	Args() string
	IncludeTransitiveNeeds() bool
}

type ApplyConfigProvider interface {
	Args() string
	PostRenderer() string
	PostRendererArgs() []string
	SkipSchemaValidation() bool
	Cascade() string
	HideNotes() bool
	TakeOwnership() bool
	SuppressOutputLineRegex() []string

	Values() []string
	Set() []string
	SkipCRDs() bool
	SkipDeps() bool
	SkipRefresh() bool
	Wait() bool
	WaitRetries() int
	WaitForJobs() bool

	IncludeTests() bool

	Suppress() []string
	SuppressSecrets() bool
	ShowSecrets() bool
	NoHooks() bool
	SuppressDiff() bool

	DetailedExitcode() bool
	StripTrailingCR() bool

	Color() bool
	NoColor() bool
	Context() int
	DiffOutput() string

	// TODO: Remove this function once Helmfile v0.x
	RetainValuesFiles() bool

	Validate() bool
	SkipCleanup() bool
	SkipDiffOnInstall() bool

	DiffArgs() string
	SyncArgs() string
	TemplateArgs() string

	DAGConfig

	concurrencyConfig
	interactive
	loggingConfig
	valuesControlMode
}

type SyncConfigProvider interface {
	Args() string
	PostRenderer() string
	SkipSchemaValidation() bool
	PostRendererArgs() []string
	HideNotes() bool
	TakeOwnership() bool
	Cascade() string

	Values() []string
	Set() []string
	SkipCRDs() bool
	SkipDeps() bool
	SkipRefresh() bool
	Wait() bool
	WaitRetries() int
	WaitForJobs() bool
	SyncArgs() string

	Validate() bool

	SkipNeeds() bool
	IncludeNeeds() bool
	IncludeTransitiveNeeds() bool
	DAGConfig

	concurrencyConfig
	interactive
	loggingConfig
	valuesControlMode
}

type DiffConfigProvider interface {
	Args() string
	PostRenderer() string
	PostRendererArgs() []string
	SkipSchemaValidation() bool
	SuppressOutputLineRegex() []string

	Values() []string
	Set() []string
	Validate() bool
	SkipCRDs() bool
	SkipDeps() bool
	SkipRefresh() bool

	IncludeTests() bool

	Suppress() []string
	SuppressSecrets() bool
	ShowSecrets() bool
	NoHooks() bool
	SuppressDiff() bool
	SkipDiffOnInstall() bool
	DiffArgs() string

	DAGConfig

	DetailedExitcode() bool
	StripTrailingCR() bool
	Color() bool
	NoColor() bool
	Context() int
	DiffOutput() string

	concurrencyConfig
	valuesControlMode
}

// TODO: Remove this function once Helmfile v0.x
type DeleteConfigProvider interface {
	Args() string
	Cascade() string

	Purge() bool
	SkipDeps() bool
	SkipRefresh() bool
	SkipCharts() bool
	DeleteWait() bool
	DeleteTimeout() int

	interactive
	loggingConfig
	concurrencyConfig
}

type DestroyConfigProvider interface {
	Args() string
	Cascade() string

	SkipDeps() bool
	SkipRefresh() bool
	SkipCharts() bool
	DeleteWait() bool
	DeleteTimeout() int

	interactive
	loggingConfig
	concurrencyConfig
}

type TestConfigProvider interface {
	Args() string

	SkipDeps() bool
	SkipRefresh() bool
	Timeout() int
	Cleanup() bool
	Logs() bool

	concurrencyConfig
}

type LintConfigProvider interface {
	Args() string

	Values() []string
	Set() []string
	SkipDeps() bool
	SkipRefresh() bool
	SkipCleanup() bool

	DAGConfig

	concurrencyConfig
}

type FetchConfigProvider interface {
	SkipDeps() bool
	SkipRefresh() bool
	OutputDir() string
	OutputDirTemplate() string

	concurrencyConfig
}

type TemplateConfigProvider interface {
	Args() string
	PostRenderer() string
	PostRendererArgs() []string
	SkipSchemaValidation() bool

	Values() []string
	Set() []string
	OutputDirTemplate() string
	Validate() bool
	SkipDeps() bool
	SkipRefresh() bool
	SkipCleanup() bool
	SkipTests() bool
	OutputDir() string
	IncludeCRDs() bool
	NoHooks() bool
	KubeVersion() string
	ShowOnly() []string
	TemplateArgs() string

	DAGConfig

	concurrencyConfig
}

type DAGConfig interface {
	SkipNeeds() bool
	IncludeNeeds() bool
	IncludeTransitiveNeeds() bool
}

type WriteValuesConfigProvider interface {
	Values() []string
	Set() []string
	OutputFileTemplate() string
	SkipDeps() bool
	SkipRefresh() bool
	SkipCleanup() bool
	IncludeTransitiveNeeds() bool

	concurrencyConfig
}

type StatusesConfigProvider interface {
	Args() string

	concurrencyConfig
}

type StateConfigProvider interface {
	EmbedValues() bool
}

type DAGConfigProvider any

type concurrencyConfig interface {
	Concurrency() int
}

type loggingConfig interface {
	Logger() *zap.SugaredLogger
}

type interactive interface {
	Interactive() bool
}

type ListConfigProvider interface {
	Output() string
	SkipCharts() bool
}

type CacheConfigProvider any

type InitConfigProvider interface {
	Force() bool
}

// reset/reuse values helm cli flags handling for apply/sync/diff
type valuesControlMode interface {
	ReuseValues() bool
	ResetValues() bool
}
