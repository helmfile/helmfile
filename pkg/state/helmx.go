package state

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/helmfile/chartify"
	"helm.sh/helm/v3/pkg/storage/driver"

	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/remote"
)

type Dependency struct {
	Chart   string `yaml:"chart"`
	Version string `yaml:"version"`
	Alias   string `yaml:"alias"`
}

func (st *HelmState) appendHelmXFlags(flags []string, release *ReleaseSpec) []string {
	for _, adopt := range release.Adopt {
		flags = append(flags, "--adopt", adopt)
	}

	return flags
}

func formatLabels(labels map[string]string) string {
	var labelsList, keys []string
	for k := range labels {
		if k == "" || slices.Contains(driver.GetSystemLabels(), k) {
			continue
		}

		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		return ""
	}

	for _, k := range keys {
		val := labels[k]
		labelsList = append(labelsList, fmt.Sprintf("%s=%s", k, val))
	}
	return strings.Join(labelsList, ",")
}

// append labels flags to helm flags, starting from helm v3.13.0
func (st *HelmState) appendLabelsFlags(flags []string, helm helmexec.Interface, release *ReleaseSpec, syncReleaseLabels bool) []string {
	if helm.IsVersionAtLeast("3.13.0") && (syncReleaseLabels || release.SyncReleaseLabels) {
		labels := formatLabels(release.Labels)
		if labels != "" {
			flags = append(flags, "--labels", labels)
		}
	}
	return flags
}

// append post-renderer flags to helm flags
func (st *HelmState) appendPostRenderFlags(flags []string, release *ReleaseSpec, postRenderer string) []string {
	switch {
	// postRenderer arg comes from cmd flag.
	case release.PostRenderer != nil && *release.PostRenderer != "":
		flags = append(flags, "--post-renderer", *release.PostRenderer)
	case postRenderer != "":
		flags = append(flags, "--post-renderer", postRenderer)
	case st.HelmDefaults.PostRenderer != nil && *st.HelmDefaults.PostRenderer != "":
		flags = append(flags, "--post-renderer", *st.HelmDefaults.PostRenderer)
	}
	return flags
}

// append post-renderer-args flags to helm flags
func (st *HelmState) appendPostRenderArgsFlags(flags []string, release *ReleaseSpec, postRendererArgs []string) []string {
	postRendererArgsFlags := []string{}
	switch {
	case len(release.PostRendererArgs) != 0:
		postRendererArgsFlags = release.PostRendererArgs
	case len(postRendererArgs) != 0:
		postRendererArgsFlags = postRendererArgs
	case len(st.HelmDefaults.PostRendererArgs) != 0:
		postRendererArgsFlags = st.HelmDefaults.PostRendererArgs
	}
	for _, arg := range postRendererArgsFlags {
		if arg != "" {
			flags = append(flags, "--post-renderer-args", arg)
		}
	}
	return flags
}

// append skip-schema-validation flags to helm flags
func (st *HelmState) appendSkipSchemaValidationFlags(flags []string, release *ReleaseSpec, skipSchemaValidation bool) []string {
	switch {
	// Check if SkipSchemaValidation is true in the release spec.
	case release.SkipSchemaValidation != nil && *release.SkipSchemaValidation:
		flags = append(flags, "--skip-schema-validation")
	// Check if skipSchemaValidation argument is true.
	case skipSchemaValidation:
		flags = append(flags, "--skip-schema-validation")
	// Check if SkipSchemaValidation is true in HelmDefaults.
	case st.HelmDefaults.SkipSchemaValidation != nil && *st.HelmDefaults.SkipSchemaValidation:
		flags = append(flags, "--skip-schema-validation")
	}
	return flags
}

// append suppress-output-line-regex flags to helm diff flags
func (st *HelmState) appendSuppressOutputLineRegexFlags(flags []string, release *ReleaseSpec, suppressOutputLineRegex []string) []string {
	suppressOutputLineRegexFlags := []string{}
	switch {
	case len(release.SuppressOutputLineRegex) != 0:
		suppressOutputLineRegexFlags = release.SuppressOutputLineRegex
	case len(suppressOutputLineRegex) != 0:
		suppressOutputLineRegexFlags = suppressOutputLineRegex
	case len(st.HelmDefaults.SuppressOutputLineRegex) != 0:
		suppressOutputLineRegexFlags = st.HelmDefaults.SuppressOutputLineRegex
	}
	for _, arg := range suppressOutputLineRegexFlags {
		if arg != "" {
			flags = append(flags, "--suppress-output-line-regex", arg)
		}
	}
	return flags
}

func (st *HelmState) appendWaitForJobsFlags(flags []string, release *ReleaseSpec, ops *SyncOpts) []string {
	switch {
	case release.WaitForJobs != nil && *release.WaitForJobs:
		flags = append(flags, "--wait-for-jobs")
	case ops != nil && ops.WaitForJobs:
		flags = append(flags, "--wait-for-jobs")
	case release.WaitForJobs == nil && st.HelmDefaults.WaitForJobs:
		flags = append(flags, "--wait-for-jobs")
	}
	return flags
}

func (st *HelmState) appendWaitFlags(flags []string, helm helmexec.Interface, release *ReleaseSpec, ops *SyncOpts) []string {
	var hasWait bool
	switch {
	case release.Wait != nil && *release.Wait:
		hasWait = true
		flags = append(flags, "--wait")
	case ops != nil && ops.Wait:
		hasWait = true
		flags = append(flags, "--wait")
	case release.Wait == nil && st.HelmDefaults.Wait:
		hasWait = true
		flags = append(flags, "--wait")
	}
	// see https://github.com/helm/helm/releases/tag/v3.15.0
	// https://github.com/helm/helm/commit/fc74964
	if hasWait && helm.IsVersionAtLeast("3.15.0") {
		switch {
		case release.WaitRetries != nil && *release.WaitRetries > 0:
			flags = append(flags, "--wait-retries", strconv.Itoa(*release.WaitRetries))
		case ops != nil && ops.WaitRetries > 0:
			flags = append(flags, "--wait-retries", strconv.Itoa(ops.WaitRetries))
		case release.WaitRetries == nil && st.HelmDefaults.WaitRetries > 0:
			flags = append(flags, "--wait-retries", strconv.Itoa(st.HelmDefaults.WaitRetries))
		}
	}
	return flags
}

// append post-renderer flags to helm flags
func (st *HelmState) appendCascadeFlags(flags []string, helm helmexec.Interface, release *ReleaseSpec, cascade string) []string {
	// see https://github.com/helm/helm/releases/tag/v3.12.1
	if !helm.IsVersionAtLeast("3.12.1") {
		return flags
	}
	switch {
	// postRenderer arg comes from cmd flag.
	case release.Cascade != nil && *release.Cascade != "":
		flags = append(flags, "--cascade", *release.Cascade)
	case cascade != "":
		flags = append(flags, "--cascade", cascade)
	case st.HelmDefaults.Cascade != nil && *st.HelmDefaults.Cascade != "":
		flags = append(flags, "--cascade", *st.HelmDefaults.Cascade)
	}
	return flags
}

// append hide-notes flags to helm flags
func (st *HelmState) appendHideNotesFlags(flags []string, helm helmexec.Interface, ops *SyncOpts) []string {
	// see https://github.com/helm/helm/releases/tag/v3.16.0
	if !helm.IsVersionAtLeast("3.16.0") {
		return flags
	}
	switch {
	case ops.HideNotes:
		flags = append(flags, "--hide-notes")
	}
	return flags
}

// append take-ownership flags to helm flags
func (st *HelmState) appendTakeOwnershipFlags(flags []string, helm helmexec.Interface, ops *SyncOpts) []string {
	// see https://github.com/helm/helm/releases/tag/v3.17.0
	if !helm.IsVersionAtLeast("3.17.0") {
		return flags
	}
	switch {
	case ops.TakeOwnership:
		flags = append(flags, "--take-ownership")
	}
	return flags
}

// append show-only flags to helm flags
func (st *HelmState) appendShowOnlyFlags(flags []string, showOnly []string) []string {
	showOnlyFlags := []string{}
	if len(showOnly) != 0 {
		showOnlyFlags = showOnly
	}
	for _, arg := range showOnlyFlags {
		if arg != "" {
			flags = append(flags, "--show-only", arg)
		}
	}
	return flags
}

type Chartify struct {
	Opts  *chartify.ChartifyOpts
	Clean func()
}

func (st *HelmState) downloadChartWithGoGetter(r *ReleaseSpec) (string, error) {
	var pathElems []string

	if r.Namespace != "" {
		pathElems = append(pathElems, r.Namespace)
	}

	if r.KubeContext != "" {
		pathElems = append(pathElems, r.KubeContext)
	}

	pathElems = append(pathElems, r.Name)

	cacheDir := filepath.Join(pathElems...)

	return st.goGetterChart(r.Chart, r.Directory, cacheDir, r.ForceGoGetter)
}

func (st *HelmState) goGetterChart(chart, dir, cacheDir string, force bool) (string, error) {
	if dir != "" && chart == "" {
		chart = dir
	}

	_, err := remote.Parse(chart)
	if err != nil {
		if force {
			return "", fmt.Errorf("Parsing url from dir failed due to error %q.\nContinuing the process assuming this is a regular Helm chart or a local dir.", err.Error())
		}
	} else {
		r := remote.NewRemote(st.logger, "", st.fs)

		fetchedDir, err := r.Fetch(chart, cacheDir)
		if err != nil {
			return "", fmt.Errorf("fetching %q: %v", chart, err)
		}

		chart = fetchedDir
	}

	return chart, nil
}

func (st *HelmState) PrepareChartify(helm helmexec.Interface, release *ReleaseSpec, chart string, workerIndex int) (*Chartify, func(), error) {
	c := &Chartify{
		Opts: &chartify.ChartifyOpts{
			WorkaroundOutputDirIssue:    true,
			EnableKustomizeAlphaPlugins: true,
			ChartVersion:                release.Version,
			Namespace:                   release.Namespace,
			ID:                          ReleaseToID(release),
		},
	}

	var filesNeedCleaning []string

	clean := func() {
		st.removeFiles(filesNeedCleaning)
	}

	var shouldRun bool

	dir := chart
	if !filepath.IsAbs(chart) {
		dir = filepath.Join(st.basePath, chart)
	}
	if stat, _ := os.Stat(dir); stat != nil && stat.IsDir() {
		if exists, err := st.fs.FileExists(filepath.Join(dir, "Chart.yaml")); err == nil && !exists {
			shouldRun = true
		}
	}

	for _, d := range release.Dependencies {
		chart := d.Chart
		if st.fs.DirectoryExistsAt(chart) {
			var err error

			// Otherwise helm-dependency-up on the temporary chart generated by chartify ends up errors like:
			//   Error: directory /tmp/chartify945964195/myapp-57fb4495cf/test/integration/charts/httpbin not found]
			// which is due to that the temporary chart is generated outside of the current working directory/basePath,
			// and therefore the relative path in `chart` points to somewhere inexistent.
			chart, err = filepath.Abs(filepath.Join(st.basePath, chart))
			if err != nil {
				return nil, clean, err
			}
		}

		c.Opts.AdhocChartDependencies = append(c.Opts.AdhocChartDependencies, chartify.ChartDependency{
			Alias:   d.Alias,
			Chart:   chart,
			Version: d.Version,
		})

		shouldRun = true
	}

	jsonPatches := release.JSONPatches
	if len(jsonPatches) > 0 {
		generatedFiles, err := st.generateTemporaryReleaseValuesFiles(release, jsonPatches, release.MissingFileHandler)
		if err != nil {
			return nil, clean, err
		}

		filesNeedCleaning = append(filesNeedCleaning, generatedFiles...)

		c.Opts.JsonPatches = append(c.Opts.JsonPatches, generatedFiles...)

		shouldRun = true
	}

	strategicMergePatches := release.StrategicMergePatches
	if len(strategicMergePatches) > 0 {
		generatedFiles, err := st.generateTemporaryReleaseValuesFiles(release, strategicMergePatches, release.MissingFileHandler)
		if err != nil {
			return nil, clean, err
		}

		c.Opts.StrategicMergePatches = append(c.Opts.StrategicMergePatches, generatedFiles...)

		filesNeedCleaning = append(filesNeedCleaning, generatedFiles...)

		shouldRun = true
	}

	transformers := release.Transformers
	if len(transformers) > 0 {
		generatedFiles, err := st.generateTemporaryReleaseValuesFiles(release, transformers, release.MissingFileHandler)
		if err != nil {
			return nil, clean, err
		}

		c.Opts.Transformers = append(c.Opts.Transformers, generatedFiles...)

		filesNeedCleaning = append(filesNeedCleaning, generatedFiles...)

		shouldRun = true
	}

	if release.ForceNamespace != "" {
		c.Opts.OverrideNamespace = release.ForceNamespace

		shouldRun = true
	}

	if shouldRun {
		st.logger.Debugf("Chartify process for %s", dir)
		generatedFiles, err := st.generateValuesFiles(helm, release, workerIndex)
		if err != nil {
			return nil, clean, err
		}

		filesNeedCleaning = append(filesNeedCleaning, generatedFiles...)

		c.Opts.ValuesFiles = generatedFiles
		setFlags, err := st.setFlags(release.SetValues)
		if err != nil {
			return nil, clean, fmt.Errorf("rendering set value entry for release %s: %v", release.Name, err)
		}
		c.Opts.SetFlags = setFlags
		c.Opts.TemplateData = st.newReleaseTemplateData(release)
		c.Opts.TemplateFuncs = st.newReleaseTemplateFuncMap(dir)

		return c, clean, nil
	}

	return nil, clean, nil
}
