package exectest

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	chart "helm.sh/helm/v4/pkg/chart/v2"

	"github.com/helmfile/helmfile/pkg/helmexec"
)

type ListKey struct {
	Filter string
	Flags  string
}

func (k ListKey) String() string {
	return fmt.Sprintf("listkey(filter=%s,flags=%s)", k.Filter, k.Flags)
}

type DiffKey struct {
	Name  string
	Chart string
	Flags string
}

type Helm struct {
	Charts               []string
	Repo                 []string
	Releases             []Release
	Deleted              []Release
	Linted               []Release
	Templated            []Release
	Lists                map[ListKey]string
	Diffs                map[DiffKey]error
	Diffed               []Release
	FailOnUnexpectedDiff bool
	FailOnUnexpectedList bool
	Version              *semver.Version

	UpdateDepsCallbacks map[string]func(string) error

	DiffMutex     *sync.Mutex
	ChartsMutex   *sync.Mutex
	ReleasesMutex *sync.Mutex

	Helm3 bool
	Helm4 bool
}

type Release struct {
	Name  string
	Flags []string
}

type Affected struct {
	Upgraded    []*Release
	Reinstalled []*Release
	Deleted     []*Release
	Failed      []*Release
}

func (helm *Helm) UpdateDeps(chart string) error {
	if strings.Contains(chart, "error") {
		return fmt.Errorf("simulated UpdateDeps failure for chart: %s", chart)
	}
	helm.Charts = append(helm.Charts, chart)

	if helm.UpdateDepsCallbacks != nil {
		callback, exists := helm.UpdateDepsCallbacks[chart]
		if exists {
			if err := callback(chart); err != nil {
				return err
			}
		}
	}
	return nil
}

func (helm *Helm) BuildDeps(name, chart string, flags ...string) error {
	if strings.Contains(chart, "error") {
		return errors.New("error")
	}
	helm.Charts = append(helm.Charts, chart)
	return nil
}

func (helm *Helm) SetExtraArgs(args ...string) {
}
func (helm *Helm) SetHelmBinary(bin string) {
}
func (helm *Helm) SetEnableLiveOutput(enableLiveOutput bool) {
}
func (helm *Helm) SetDisableForceUpdate(forceUpdate bool) {
}
func (helm *Helm) SkipSchemaValidation(skipSchemaValidation bool) {
}
func (helm *Helm) AddRepo(name, repository, cafile, certfile, keyfile, username, password string, managed string, passCredentials, skipTLSVerify bool) error {
	helm.Repo = []string{name, repository, cafile, certfile, keyfile, username, password, managed, fmt.Sprintf("%v", passCredentials), fmt.Sprintf("%v", skipTLSVerify)}
	return nil
}
func (helm *Helm) UpdateRepo() error {
	return nil
}
func (helm *Helm) RegistryLogin(name, username, password, caFile, certFile, keyFile string, skipTLSVerify bool) error {
	return nil
}
func (helm *Helm) SyncRelease(context helmexec.HelmContext, name, chart, namespace string, flags ...string) error {
	if strings.Contains(name, "forbidden") {
		releaseExists := false
		for _, release := range helm.Releases {
			if release.Name == name {
				releaseExists = true
			}
		}
		releaseDeleted := false
		for _, release := range helm.Deleted {
			if release.Name == name {
				releaseDeleted = true
			}
		}
		// Only fail if the release is present in the helm.Releases to simulate a forbidden update if it exists
		if releaseExists && !releaseDeleted {
			return fmt.Errorf("cannot patch %q with kind StatefulSet: StatefulSet.apps %q is invalid: spec: Forbidden: updates to statefulset spec for fields other than 'replicas', 'ordinals', 'template', 'updateStrategy', 'persistentVolumeClaimRetentionPolicy' and 'minReadySeconds' are forbidden", name, name)
		}
	} else if strings.Contains(name, "error") {
		return errors.New("error")
	}
	helm.sync(helm.ReleasesMutex, func() {
		helm.Releases = append(helm.Releases, Release{Name: name, Flags: flags})
	})
	helm.sync(helm.ChartsMutex, func() {
		helm.Charts = append(helm.Charts, chart)
	})

	return nil
}
func (helm *Helm) DiffRelease(context helmexec.HelmContext, name, chart, namespace string, suppressDiff bool, flags ...string) error {
	if helm.DiffMutex != nil {
		helm.DiffMutex.Lock()
	}
	helm.Diffed = append(helm.Diffed, Release{Name: name, Flags: flags})
	if helm.DiffMutex != nil {
		helm.DiffMutex.Unlock()
	}

	if helm.Diffs == nil {
		return nil
	}

	key := DiffKey{Name: name, Chart: chart, Flags: strings.Join(flags, " ")}
	err, ok := helm.Diffs[key]
	if !ok && helm.FailOnUnexpectedDiff {
		return fmt.Errorf("unexpected diff with key: %v", key)
	}
	return err
}
func (helm *Helm) ReleaseStatus(context helmexec.HelmContext, release string, flags ...string) error {
	if strings.Contains(release, "notFound") {
		return errors.New("Error: release: not found")
	}
	if strings.Contains(release, "error") {
		return errors.New("error")
	}
	helm.Releases = append(helm.Releases, Release{Name: release, Flags: flags})
	return nil
}
func (helm *Helm) DeleteRelease(context helmexec.HelmContext, name string, flags ...string) error {
	if strings.Contains(name, "error") {
		return errors.New("error")
	}
	helm.Deleted = append(helm.Deleted, Release{Name: name, Flags: flags})
	return nil
}
func (helm *Helm) List(context helmexec.HelmContext, filter string, flags ...string) (string, error) {
	key := ListKey{Filter: filter, Flags: strings.Join(flags, " ")}

	if helm.Lists == nil {
		return "dummy non-empty helm-list output", nil
	}

	res, ok := helm.Lists[key]
	if !ok && helm.FailOnUnexpectedList {
		var keys []string
		for k := range helm.Lists {
			keys = append(keys, k.String())
		}
		return "", fmt.Errorf("unexpected list key: %v not found in %v", key, strings.Join(keys, ", "))
	}
	return res, nil
}
func (helm *Helm) DecryptSecret(context helmexec.HelmContext, name string, flags ...string) (string, error) {
	return "", nil
}
func (helm *Helm) TestRelease(context helmexec.HelmContext, name string, flags ...string) error {
	if strings.Contains(name, "error") {
		return errors.New("error")
	}
	helm.Releases = append(helm.Releases, Release{Name: name, Flags: flags})
	return nil
}
func (helm *Helm) Fetch(chart string, flags ...string) error {
	return nil
}
func (helm *Helm) Lint(name, chart string, flags ...string) error {
	if strings.Contains(name, "error") {
		return errors.New("error")
	}
	helm.Linted = append(helm.Linted, Release{Name: name, Flags: flags})
	return nil
}
func (helm *Helm) TemplateRelease(name, chart string, flags ...string) error {
	if strings.Contains(name, "error") {
		return errors.New("error")
	}
	helm.Templated = append(helm.Templated, Release{Name: name, Flags: flags})
	return nil
}
func (helm *Helm) ChartPull(chart string, path string, flags ...string) error {
	return nil
}
func (helm *Helm) ChartExport(chart string, path string) error {
	return nil
}
func (helm *Helm) IsHelm3() bool {
	// Priority order:
	// 1. If Version is explicitly set, use that
	if helm.Version != nil {
		return helm.Version.Major() == 3
	}

	// 2. Check explicit struct field settings (for unit tests)
	if helm.Helm3 {
		return true
	}
	if helm.Helm4 {
		return false
	}

	// 3. Check environment variable (for CI matrix testing)
	if IsHelm4Enabled() {
		return false
	}

	// 4. Default to Helm 4 (newer version)
	return false
}

func (helm *Helm) IsHelm4() bool {
	// Priority order:
	// 1. If Version is explicitly set, use that
	if helm.Version != nil {
		return helm.Version.Major() == 4
	}

	// 2. Check explicit struct field settings (for unit tests)
	if helm.Helm4 {
		return true
	}
	if helm.Helm3 {
		return false
	}

	// 3. Check environment variable (for CI matrix testing)
	if IsHelm4Enabled() {
		return true
	}

	// 4. Default to Helm 4 (newer version)
	return true
}

func (helm *Helm) GetVersion() helmexec.Version {
	return helmexec.Version{
		Major: int(helm.Version.Major()),
		Minor: int(helm.Version.Minor()),
		Patch: int(helm.Version.Patch()),
	}
}

func (helm *Helm) IsVersionAtLeast(versionStr string) bool {
	if helm.Version == nil {
		return false
	}

	ver := semver.MustParse(versionStr)
	return helm.Version.Equal(ver) || helm.Version.GreaterThan(ver)
}

func (helm *Helm) sync(m *sync.Mutex, f func()) {
	if m != nil {
		m.Lock()
		defer m.Unlock()
	}

	f()
}

func (helm *Helm) ShowChart(chartPath string) (chart.Metadata, error) {
	switch chartPath {
	case "../../foo-bar":
		return chart.Metadata{Version: "3.2.0"}, nil
	default:
		return chart.Metadata{}, errors.New("fake test error")
	}
}

// IsHelm4Enabled detects the installed Helm version by executing the helm binary.
// It returns true if Helm 4.x is installed, false for Helm 3.x or earlier.
// Falls back to environment variable HELMFILE_HELM4 if helm binary is not available.
func IsHelm4Enabled() bool {
	// First try to detect actual Helm version
	helmBinary := os.Getenv("HELM_BIN")
	if helmBinary == "" {
		helmBinary = "helm"
	}

	// Create a simple runner for executing helm version
	runner := &simpleRunner{}
	version, err := helmexec.GetHelmVersion(helmBinary, runner)
	if err == nil && version != nil {
		return version.Major() == 4
	}

	// Fallback to environment variable for CI/testing scenarios where helm might not be available
	return os.Getenv("HELMFILE_HELM4") == "1"
}

// simpleRunner is a minimal implementation of helmexec.Runner for version detection
type simpleRunner struct{}

func (r *simpleRunner) Execute(cmd string, args []string, env map[string]string, enableLiveOutput bool) ([]byte, error) {
	command := exec.Command(cmd, args...)
	if env != nil {
		command.Env = append(os.Environ(), mapToEnv(env)...)
	}
	return command.CombinedOutput()
}

func (r *simpleRunner) ExecuteStdIn(cmd string, args []string, env map[string]string, stdin io.Reader) ([]byte, error) {
	command := exec.Command(cmd, args...)
	if env != nil {
		command.Env = append(os.Environ(), mapToEnv(env)...)
	}
	command.Stdin = stdin
	return command.CombinedOutput()
}

func mapToEnv(m map[string]string) []string {
	var env []string
	for k, v := range m {
		env = append(env, k+"="+v)
	}
	return env
}
