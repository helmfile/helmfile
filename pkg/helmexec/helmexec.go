package helmexec

import chart "helm.sh/helm/v4/pkg/chart/v2"

// Version represents the version of helm
type Version struct {
	Major int
	Minor int
	Patch int
}

// Interface for executing helm commands
type Interface interface {
	SetExtraArgs(args ...string)
	SetHelmBinary(bin string)
	SetEnableLiveOutput(enableLiveOutput bool)
	SetDisableForceUpdate(forceUpdate bool)

	AddRepo(name, repository, cafile, certfile, keyfile, username, password string, managed string, passCredentials, skipTLSVerify bool) error
	UpdateRepo() error
	RegistryLogin(name, username, password, caFile, certFile, keyFile string, skipTLSVerify bool) error
	BuildDeps(name, chart string, flags ...string) error
	UpdateDeps(chart string) error
	SyncRelease(context HelmContext, name, chart, namespace string, flags ...string) error
	DiffRelease(context HelmContext, name, chart, namespace string, suppressDiff bool, flags ...string) error
	TemplateRelease(name, chart string, flags ...string) error
	Fetch(chart string, flags ...string) error
	ChartPull(chart string, path string, flags ...string) error
	ChartExport(chart string, path string) error
	Lint(name, chart string, flags ...string) error
	Unittest(name, chart string, flags ...string) error
	ReleaseStatus(context HelmContext, name string, flags ...string) error
	DeleteRelease(context HelmContext, name string, flags ...string) error
	TestRelease(context HelmContext, name string, flags ...string) error
	List(context HelmContext, filter string, flags ...string) (string, error)
	DecryptSecret(context HelmContext, name string, flags ...string) (string, error)
	IsHelm3() bool
	IsHelm4() bool
	GetVersion() Version
	IsVersionAtLeast(versionStr string) bool
	ShowChart(chart string) (chart.Metadata, error)
}

type DependencyUpdater interface {
	UpdateDeps(chart string) error
	IsHelm3() bool
	IsHelm4() bool
}

// ChartInspector is a capability interface exposing `helm show chart` with
// arbitrary flags. It is implemented by helmexec's concrete execer and by
// stubs in tests. Callers should type-assert Interface to ChartInspector and
// fall back gracefully when the implementation does not satisfy it, so that
// third-party implementations of helmexec.Interface keep compiling. See
// state.HelmState.resolveOCIConstraintVersion for the usage pattern.
type ChartInspector interface {
	// ShowChartWithFlags runs `helm show chart <chart> [flags...]` and returns
	// the parsed Chart.yaml metadata. Unlike ShowChart, callers pass flags
	// such as --version, --plain-http, or --registry-config so that
	// constraint versions (e.g. "~1", "^2.0.0") can be resolved against an
	// OCI registry to a concrete semver.
	ShowChartWithFlags(chart string, flags ...string) (chart.Metadata, error)
}
