package testutil

import (
	"github.com/Masterminds/semver/v3"
	chart "helm.sh/helm/v4/pkg/chart/v2"

	"github.com/helmfile/helmfile/pkg/helmexec"
)

type noCallHelmExec struct {
}

type HelmExec struct {
	*noCallHelmExec
	isHelm3 bool
	isHelm4 bool
}

func NewHelmExec(isHelm4 bool) *HelmExec {
	return &HelmExec{noCallHelmExec: &noCallHelmExec{}, isHelm3: !isHelm4, isHelm4: isHelm4}
}

type VersionHelmExec struct {
	*noCallHelmExec
	version string
}

func NewVersionHelmExec(version string) *VersionHelmExec {
	return &VersionHelmExec{noCallHelmExec: &noCallHelmExec{}, version: version}
}

func (helm *HelmExec) IsHelm3() bool {
	return helm.isHelm3
}

func (helm *HelmExec) IsHelm4() bool {
	return helm.isHelm4
}

func (helm *VersionHelmExec) IsHelm3() bool {
	currentSemVer := semver.MustParse(helm.version)
	return currentSemVer.Major() == 3
}

func (helm *VersionHelmExec) IsHelm4() bool {
	currentSemVer := semver.MustParse(helm.version)
	return currentSemVer.Major() == 4
}

func (helm *VersionHelmExec) IsVersionAtLeast(ver string) bool {
	currentSemVer := semver.MustParse(helm.version)
	verSemVer := semver.MustParse(ver)
	return currentSemVer.Compare(verSemVer) >= 0
}

func (helm *noCallHelmExec) doPanic() {
	panic("unexpected call to helm")
}

func (helm *noCallHelmExec) TemplateRelease(name, chart string, flags ...string) error {
	helm.doPanic()
	return nil
}
func (helm *noCallHelmExec) ChartPull(chart string, path string, flags ...string) error {
	helm.doPanic()
	return nil
}
func (helm *noCallHelmExec) ChartExport(chart string, path string) error {
	helm.doPanic()
	return nil
}
func (helm *noCallHelmExec) UpdateDeps(chart string) error {
	helm.doPanic()
	return nil
}

func (helm *noCallHelmExec) BuildDeps(name, chart string, flags ...string) error {
	helm.doPanic()
	return nil
}

func (helm *noCallHelmExec) SetExtraArgs(args ...string) {
	helm.doPanic()
}
func (helm *noCallHelmExec) SetHelmBinary(bin string) {
	helm.doPanic()
}
func (helm *noCallHelmExec) SetEnableLiveOutput(enableLiveOutput bool) {
	helm.doPanic()
}
func (helm *noCallHelmExec) SetDisableForceUpdate(forceUpdate bool) {
	helm.doPanic()
}

func (helm *noCallHelmExec) AddRepo(name, repository, cafile, certfile, keyfile, username, password string, managed string, passCredentials, skipTLSVerify bool) error {
	helm.doPanic()
	return nil
}
func (helm *noCallHelmExec) UpdateRepo() error {
	helm.doPanic()
	return nil
}
func (helm *noCallHelmExec) RegistryLogin(name, username, password, caFile, certFile, keyFile string, skipTLSVerify bool) error {
	helm.doPanic()
	return nil
}
func (helm *noCallHelmExec) SyncRelease(context helmexec.HelmContext, name, chart, namespace string, flags ...string) error {
	helm.doPanic()
	return nil
}
func (helm *noCallHelmExec) DiffRelease(context helmexec.HelmContext, name, chart, namespace string, suppressDiff bool, flags ...string) error {
	helm.doPanic()
	return nil
}
func (helm *noCallHelmExec) ReleaseStatus(context helmexec.HelmContext, release string, flags ...string) error {
	helm.doPanic()
	return nil
}
func (helm *noCallHelmExec) DeleteRelease(context helmexec.HelmContext, name string, flags ...string) error {
	helm.doPanic()
	return nil
}

func (helm *noCallHelmExec) List(context helmexec.HelmContext, filter string, flags ...string) (string, error) {
	helm.doPanic()
	return "", nil
}

func (helm *noCallHelmExec) DecryptSecret(context helmexec.HelmContext, name string, flags ...string) (string, error) {
	helm.doPanic()
	return "", nil
}
func (helm *noCallHelmExec) TestRelease(context helmexec.HelmContext, name string, flags ...string) error {
	helm.doPanic()
	return nil
}
func (helm *noCallHelmExec) Fetch(chart string, flags ...string) error {
	helm.doPanic()
	return nil
}
func (helm *noCallHelmExec) Lint(name, chart string, flags ...string) error {
	helm.doPanic()
	return nil
}
func (helm *noCallHelmExec) IsHelm3() bool {
	helm.doPanic()
	return false
}

func (helm *noCallHelmExec) IsHelm4() bool {
	helm.doPanic()
	return false
}

func (helm *noCallHelmExec) GetVersion() helmexec.Version {
	helm.doPanic()
	return helmexec.Version{}
}

func (helm *noCallHelmExec) IsVersionAtLeast(versionStr string) bool {
	helm.doPanic()
	return false
}

func (helm *noCallHelmExec) ShowChart(chartPath string) (chart.Metadata, error) {
	helm.doPanic()
	return chart.Metadata{}, nil
}
