package app

import "go.uber.org/zap"

type ConfigProvider interface {
	Args() string
	HelmBinary() string
	EnableLiveOutput() bool

	FileOrDir() string
	KubeContext() string
	Namespace() string
	Chart() string
	Selectors() []string
	StateValuesSet() map[string]interface{}
	StateValuesFiles() []string
	Env() string

	loggingConfig
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

	Color() bool
	NoColor() bool
	Context() int
	DiffOutput() string

	Validate() bool
	SkipCleanup() bool
	SkipDiffOnInstall() bool

	DAGConfig

	concurrencyConfig
	interactive
	loggingConfig
	valuesControlMode
}

type SyncConfigProvider interface {
	Args() string

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

	DAGConfig

	DetailedExitcode() bool
	Color() bool
	NoColor() bool
	Context() int
	DiffOutput() string

	concurrencyConfig
	valuesControlMode
}

type DestroyConfigProvider interface {
	Args() string

	SkipDeps() bool

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

	concurrencyConfig
}

type TemplateConfigProvider interface {
	Args() string

	Values() []string
	Set() []string
	OutputDirTemplate() string
	Validate() bool
	SkipDeps() bool
	SkipCleanup() bool
	SkipTests() bool
	OutputDir() string
	IncludeCRDs() bool

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

type CacheConfigProvider interface{}

// when enable reuse-values, reuse the last release's values and merge in any overrides values.
type valuesControlMode interface {
	ReuseValues() bool
}
