package state

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/helmfile/chartify"

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
	switch {
	case len(release.PostRendererArgs) != 0:
		for _, arg := range release.PostRendererArgs {
			if arg != "" {
				flags = append(flags, "--post-renderer-args", arg)
			}
		}
	case len(postRendererArgs) != 0:
		for _, arg := range postRendererArgs {
			if arg != "" {
				flags = append(flags, "--post-renderer-args", arg)
			}
		}
	case len(st.HelmDefaults.PostRendererArgs) != 0:
		for _, arg := range st.HelmDefaults.PostRendererArgs {
			if arg != "" {
				flags = append(flags, "--post-renderer-args", arg)
			}
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

func (st *HelmState) appendWaitFlags(flags []string, release *ReleaseSpec, ops *SyncOpts) []string {
	switch {
	case release.Wait != nil && *release.Wait:
		flags = append(flags, "--wait")
	case ops != nil && ops.Wait:
		flags = append(flags, "--wait")
	case release.Wait == nil && st.HelmDefaults.Wait:
		flags = append(flags, "--wait")
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
