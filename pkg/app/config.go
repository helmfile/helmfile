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

	FileOrDir() string
	KubeContext() string
	Namespace() string
	Chart() string
	Selectors() []string
	StateValuesSet() map[string]any
	StateValuesFiles() []string
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
	Cascade() string

	Values() []string
	Set() []string
	SkipCRDs() bool
	SkipDeps() bool
	Wait() bool
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

	DAGConfig

	concurrencyConfig
	interactive
	loggingConfig
	valuesControlMode
}

type SyncConfigProvider interface {
	Args() string
	PostRenderer() string
	Cascade() string

	Values() []string
	Set() []string
	SkipCRDs() bool
	SkipDeps() bool
	Wait() bool
	WaitForJobs() bool

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

	Values() []string
	Set() []string
	Validate() bool
	SkipCRDs() bool
	SkipDeps() bool

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
	SkipCharts() bool
	Wait() bool

	interactive
	loggingConfig
	concurrencyConfig
}

type DestroyConfigProvider interface {
	Args() string
	Cascade() string

	SkipDeps() bool
	SkipCharts() bool
	Wait() bool

	interactive
	loggingConfig
	concurrencyConfig
}

type TestConfigProvider interface {
	Args() string

	SkipDeps() bool
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
	SkipCleanup() bool

	DAGConfig

	concurrencyConfig
}

type FetchConfigProvider interface {
	SkipDeps() bool
	OutputDir() string
	OutputDirTemplate() string

	concurrencyConfig
}

type TemplateConfigProvider interface {
	Args() string
	PostRenderer() string

	Values() []string
	Set() []string
	OutputDirTemplate() string
	Validate() bool
	SkipDeps() bool
	SkipCleanup() bool
	SkipTests() bool
	OutputDir() string
	IncludeCRDs() bool
	KubeVersion() string

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
