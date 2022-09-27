package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	goversion "github.com/hashicorp/go-version"
	"github.com/r3labs/diff"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/helmfile/pkg/app/version"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/maputil"
)

type ChartMeta struct {
	Name string `yaml:"name"`
}

type unresolvedChartDependency struct {
	// ChartName identifies the dependant chart. In Helmfile, ChartName for `chart: stable/envoy` would be just `envoy`.
	// It can't be collided with other charts referenced in the same helmfile spec.
	// That is, collocating `chart: incubator/foo` and `chart: stable/foo` isn't allowed. Name them differently for a work-around.
	ChartName string `yaml:"name"`
	// Repository contains the URL for the helm chart repository that hosts the chart identified by ChartName
	Repository string `yaml:"repository"`
	// VersionConstraint is the version constraint of the dependent chart. "*" means the latest version.
	VersionConstraint string `yaml:"version"`
}

type ResolvedChartDependency struct {
	// ChartName identifies the dependant chart. In Helmfile, ChartName for `chart: stable/envoy` would be just `envoy`.
	// It can't be collided with other charts referenced in the same helmfile spec.
	// That is, collocating `chart: incubator/foo` and `chart: stable/foo` isn't allowed. Name them differently for a work-around.
	ChartName string `yaml:"name"`
	// Repository contains the URL for the helm chart repository that hosts the chart identified by ChartName
	Repository string `yaml:"repository"`
	// Version is the version number of the dependent chart.
	// In the context of helmfile this can be omitted. When omitted, it is considered `*` which results helm/helmfile fetching the latest version.
	Version string `yaml:"version"`
}

type UnresolvedDependencies struct {
	deps map[string][]unresolvedChartDependency
}

type ChartRequirements struct {
	UnresolvedDependencies []unresolvedChartDependency `yaml:"dependencies"`
}

type ChartLockedRequirements struct {
	Version              string                    `yaml:"version"`
	ResolvedDependencies []ResolvedChartDependency `yaml:"dependencies"`
	Digest               string                    `yaml:"digest"`
	Generated            string                    `yaml:"generated"`
}

func (d *UnresolvedDependencies) Add(chart, url, versionConstraint string) error {
	dep := unresolvedChartDependency{
		ChartName:         chart,
		Repository:        url,
		VersionConstraint: versionConstraint,
	}
	return d.add(dep)
}

func (d *UnresolvedDependencies) add(dep unresolvedChartDependency) error {
	deps := d.deps[dep.ChartName]
	if deps == nil {
		deps = []unresolvedChartDependency{dep}
	} else {
		deps = append(deps, dep)
	}
	d.deps[dep.ChartName] = deps
	return nil
}

func (d *UnresolvedDependencies) ToChartRequirements() *ChartRequirements {
	deps := []unresolvedChartDependency{}

	for _, ds := range d.deps {
		for _, d := range ds {
			if d.VersionConstraint == "" {
				d.VersionConstraint = "*"
			}
			deps = append(deps, d)
		}
	}

	return &ChartRequirements{UnresolvedDependencies: deps}
}

type ResolvedDependencies struct {
	deps map[string][]ResolvedChartDependency
}

// nolint: unparam
func (d *ResolvedDependencies) add(dep ResolvedChartDependency) error {
	deps := d.deps[dep.ChartName]
	if deps == nil {
		deps = []ResolvedChartDependency{dep}
	} else {
		deps = append(deps, dep)
	}
	d.deps[dep.ChartName] = deps
	return nil
}

func (d *ResolvedDependencies) Get(chart, versionConstraint string) (string, error) {
	if versionConstraint == "" {
		versionConstraint = "*"
	}

	deps, exists := d.deps[chart]
	if exists {
		for _, dep := range deps {
			constraint, err := semver.NewConstraint(versionConstraint)
			if err != nil {
				return "", err
			}
			version, err := semver.NewVersion(dep.Version)
			if err != nil {
				return "", err
			}
			if constraint.Check(version) {
				return dep.Version, nil
			}
		}
	}
	return "", fmt.Errorf("no resolved dependency found for \"%s\", running \"helmfile deps\" may resolve the issue", chart)
}

func (st *HelmState) mergeLockedDependencies() (*HelmState, error) {
	filename, unresolved, err := getUnresolvedDependenciess(st)
	if err != nil {
		return nil, err
	}

	if len(unresolved.deps) == 0 {
		return st, nil
	}

	depMan := NewChartDependencyManager(filename, st.logger)

	if st.fs.ReadFile != nil {
		depMan.readFile = st.fs.ReadFile
	}

	return resolveDependencies(st, depMan, unresolved)
}

func resolveDependencies(st *HelmState, depMan *chartDependencyManager, unresolved *UnresolvedDependencies) (*HelmState, error) {
	resolved, lockfileExists, err := depMan.Resolve(unresolved)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve %d deps: %v", len(unresolved.deps), err)
	}
	if !lockfileExists {
		return st, nil
	}

	repoToURL := map[string]string{}

	for _, r := range st.Repositories {
		repoToURL[r.Name] = r.URL
	}

	updated := *st
	for i, r := range updated.Releases {
		repo, chart, ok := resolveRemoteChart(r.Chart)
		if !ok {
			continue
		}

		_, ok = repoToURL[repo]
		// Skip this chart from dependency management, as there's no matching `repository` in the helmfile state,
		// which may imply that this is a local chart within a directory, like `charts/myapp`
		if !ok {
			continue
		}

		ver, err := resolved.Get(chart, r.Version)
		if err != nil {
			return nil, err
		}

		updated.Releases[i].Version = ver
	}

	return &updated, nil
}

func (st *HelmState) updateDependenciesInTempDir(shell helmexec.DependencyUpdater, tempDir func(string, string) (string, error)) (*HelmState, error) {
	filename, unresolved, err := getUnresolvedDependenciess(st)
	if err != nil {
		return nil, err
	}

	if len(unresolved.deps) == 0 {
		st.logger.Warnf("There are no repositories defined in your helmfile.yaml.\nThis means helmfile cannot update your dependencies or create a lock file.\nSee https://github.com/roboll/helmfile/issues/878 for more information.")
		return st, nil
	}

	d, err := tempDir("", "")
	if err != nil {
		return nil, fmt.Errorf("unable to create dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(d)
	}()

	return updateDependencies(st, shell, unresolved, filename, d)
}

func getUnresolvedDependenciess(st *HelmState) (string, *UnresolvedDependencies, error) {
	repoToURL := map[string]string{}

	for _, r := range st.Repositories {
		repoToURL[r.Name] = r.URL
	}

	unresolved := &UnresolvedDependencies{deps: map[string][]unresolvedChartDependency{}}
	// if err := unresolved.Add("stable/envoy", "https://kubernetes-charts.storage.googleapis.com", ""); err != nil {
	//	 panic(err)
	// }

	for _, r := range st.Releases {
		repo, chart, ok := resolveRemoteChart(r.Chart)
		if !ok {
			continue
		}

		url, ok := repoToURL[repo]
		// Skip this chart from dependency management, as there's no matching `repository` in the helmfile state,
		// which may imply that this is a local chart within a directory, like `charts/myapp`
		if !ok {
			continue
		}

		if err := unresolved.Add(chart, url, r.Version); err != nil {
			return "", nil, err
		}
	}

	filename := filepath.Base(st.FilePath)
	filename = strings.TrimSuffix(filename, ".gotmpl")
	filename = strings.TrimSuffix(filename, ".yaml")
	filename = strings.TrimSuffix(filename, ".yml")

	return filename, unresolved, nil
}

func updateDependencies(st *HelmState, shell helmexec.DependencyUpdater, unresolved *UnresolvedDependencies, filename, wd string) (*HelmState, error) {
	depMan := NewChartDependencyManager(filename, st.logger)

	_, err := depMan.Update(shell, wd, unresolved)
	if err != nil {
		return nil, fmt.Errorf("unable to update %d deps: %v", len(unresolved.deps), err)
	}

	return resolveDependencies(st, depMan, unresolved)
}

type chartDependencyManager struct {
	Name string

	logger *zap.SugaredLogger

	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
}

// nolint: golint
func NewChartDependencyManager(name string, logger *zap.SugaredLogger) *chartDependencyManager {
	return &chartDependencyManager{
		Name:      name,
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
		logger:    logger,
	}
}

func (m *chartDependencyManager) lockFileName() string {
	return fmt.Sprintf("%s.lock", m.Name)
}

func (m *chartDependencyManager) Update(shell helmexec.DependencyUpdater, wd string, unresolved *UnresolvedDependencies) (*ResolvedDependencies, error) {
	if shell.IsHelm3() {
		return m.updateHelm3(shell, wd, unresolved)
	}
	return m.updateHelm2(shell, wd, unresolved)
}

func (m *chartDependencyManager) updateHelm3(shell helmexec.DependencyUpdater, wd string, unresolved *UnresolvedDependencies) (*ResolvedDependencies, error) {
	// Generate `Chart.yaml` of the temporary local chart
	chartMetaContent := fmt.Sprintf("name: %s\nversion: 1.0.0\napiVersion: v2\n", m.Name)

	// Generate `requirements.yaml` of the temporary local chart from the helmfile state
	reqsContent, err := maputil.YamlMarshal(unresolved.ToChartRequirements())
	if err != nil {
		return nil, err
	}
	if err := m.writeBytes(filepath.Join(wd, "Chart.yaml"), []byte(chartMetaContent+string(reqsContent))); err != nil {
		return nil, err
	}

	return m.doUpdate("Chart.lock", unresolved, shell, wd)
}

func (m *chartDependencyManager) updateHelm2(shell helmexec.DependencyUpdater, wd string, unresolved *UnresolvedDependencies) (*ResolvedDependencies, error) {
	// Generate `Chart.yaml` of the temporary local chart
	if err := m.writeBytes(filepath.Join(wd, "Chart.yaml"), []byte(fmt.Sprintf("name: %s\nversion: 1.0.0\n", m.Name))); err != nil {
		return nil, err
	}

	// Generate `requirements.yaml` of the temporary local chart from the helmfile state
	reqsContent, err := maputil.YamlMarshal(unresolved.ToChartRequirements())
	if err != nil {
		return nil, err
	}
	if err := m.writeBytes(filepath.Join(wd, "requirements.yaml"), reqsContent); err != nil {
		return nil, err
	}

	return m.doUpdate("requirements.lock", unresolved, shell, wd)
}

func (m *chartDependencyManager) doUpdate(chartLockFile string, unresolved *UnresolvedDependencies, shell helmexec.DependencyUpdater, wd string) (*ResolvedDependencies, error) {
	// Generate `requirements.lock` of the temporary local chart by coping `<basename>.lock`
	lockFile := m.lockFileName()

	originalLockFileContent, err := m.readBytes(lockFile)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if shell.IsHelm3() && originalLockFileContent != nil {
		if err := m.writeBytes(filepath.Join(wd, chartLockFile), originalLockFileContent); err != nil {
			return nil, err
		}
	}

	// Update the lock file by running `helm dependency update`
	if err := shell.UpdateDeps(wd); err != nil {
		return nil, err
	}

	updatedLockFileContent, err := m.readBytes(filepath.Join(wd, chartLockFile))
	if err != nil {
		return nil, err
	}

	// Sort requirements alphabetically by name.
	lockedReqs := &ChartLockedRequirements{}
	if err := yaml.Unmarshal(updatedLockFileContent, lockedReqs); err != nil {
		return nil, err
	}

	sort.Slice(lockedReqs.ResolvedDependencies, func(i, j int) bool {
		return lockedReqs.ResolvedDependencies[i].ChartName < lockedReqs.ResolvedDependencies[j].ChartName
	})

	// Don't update lock file if no dependency updated.
	if !shell.IsHelm3() && originalLockFileContent != nil {
		originalLockedReqs := &ChartLockedRequirements{}
		if err := yaml.Unmarshal(originalLockFileContent, originalLockedReqs); err != nil {
			return nil, err
		}

		changes, err := diff.Diff(originalLockedReqs.ResolvedDependencies, lockedReqs.ResolvedDependencies)

		if err != nil {
			return nil, err
		}

		if len(changes) == 0 {
			lockedReqs.Generated = originalLockedReqs.Generated
		}
	}

	lockedReqs.Version = version.Version

	updatedLockFileContent, err = maputil.YamlMarshal(lockedReqs)

	if err != nil {
		return nil, err
	}

	// Commit the lock file if and only if everything looks ok
	if err := m.writeBytes(lockFile, updatedLockFileContent); err != nil {
		return nil, err
	}

	resolved, _, err := m.Resolve(unresolved)
	return resolved, err
}

func (m *chartDependencyManager) Resolve(unresolved *UnresolvedDependencies) (*ResolvedDependencies, bool, error) {
	updatedLockFileContent, err := m.readBytes(m.lockFileName())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	// Load resolved dependencies into memory
	lockedReqs := &ChartLockedRequirements{}
	if err := yaml.Unmarshal(updatedLockFileContent, lockedReqs); err != nil {
		return nil, false, err
	}

	// Make sure go run main.go works and compatible with old lock files.
	if version.Version != "" && lockedReqs.Version != "" {
		lockedVersion, err := goversion.NewVersion(lockedReqs.Version)

		if err != nil {
			return nil, false, err
		}

		currentVersion, err := goversion.NewVersion(version.Version)

		if err != nil {
			return nil, false, err
		}

		if currentVersion.LessThan(lockedVersion) {
			return nil, false, fmt.Errorf("the lockfile was created by Helmfile %s, which is newer than current %s; Please upgrade to Helmfile %s or greater", lockedVersion.Original(), currentVersion.Original(), lockedVersion.Original())
		}
	}

	resolved := &ResolvedDependencies{deps: map[string][]ResolvedChartDependency{}}
	for _, d := range lockedReqs.ResolvedDependencies {
		if err := resolved.add(d); err != nil {
			return nil, false, err
		}
	}

	return resolved, true, nil
}

func (m *chartDependencyManager) readBytes(filename string) ([]byte, error) {
	bytes, err := m.readFile(filename)
	if err != nil {
		return nil, err
	}
	m.logger.Debugf("readBytes: read from %s:\n%s", filename, bytes)
	return bytes, nil
}

func (m *chartDependencyManager) writeBytes(filename string, data []byte) error {
	err := m.writeFile(filename, data, 0644)
	if err != nil {
		return err
	}
	m.logger.Debugf("writeBytes: wrote to %s:\n%s", filename, data)
	return nil
}
