package helmexec

import "helm.sh/helm/v3/pkg/chart"

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
	RegistryLogin(name string, username string, password string) error
	BuildDeps(name, chart string, flags ...string) error
	UpdateDeps(chart string) error
	SyncRelease(context HelmContext, name, chart string, flags ...string) error
	DiffRelease(context HelmContext, name, chart string, suppressDiff bool, flags ...string) error
	TemplateRelease(name, chart string, flags ...string) error
	Fetch(chart string, flags ...string) error
	ChartPull(chart string, path string, flags ...string) error
	ChartExport(chart string, path string, flags ...string) error
	Lint(name, chart string, flags ...string) error
	ReleaseStatus(context HelmContext, name string, flags ...string) error
	DeleteRelease(context HelmContext, name string, flags ...string) error
	TestRelease(context HelmContext, name string, flags ...string) error
	List(context HelmContext, filter string, flags ...string) (string, error)
	DecryptSecret(context HelmContext, name string, flags ...string) (string, error)
	IsHelm3() bool
	GetVersion() Version
	IsVersionAtLeast(versionStr string) bool
	ShowChart(chart string) (chart.Metadata, error)
}

type DependencyUpdater interface {
	UpdateDeps(chart string) error
	IsHelm3() bool
}
