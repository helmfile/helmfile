package state

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/imdario/mergo"
	"github.com/tatsushid/go-prettytable"
	"github.com/variantdev/chartify"
	"github.com/variantdev/vals"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/event"
	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/maputil"
	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/tmpl"
)

const (
	// EmptyTimeout represents the `--timeout` value passed to helm commands not being specified via helmfile flags.
	// This is used by an interim solution to make the urfave/cli command report to the helmfile internal about that the
	// --timeout flag is missingl
	EmptyTimeout = -1
)

// ReleaseSetSpec is release set spec
type ReleaseSetSpec struct {
	DefaultHelmBinary string `yaml:"helmBinary,omitempty"`

	// DefaultValues is the default values to be overrode by environment values and command-line overrides
	DefaultValues []interface{} `yaml:"values,omitempty"`

	Environments map[string]EnvironmentSpec `yaml:"environments,omitempty"`

	Bases               []string          `yaml:"bases,omitempty"`
	HelmDefaults        HelmSpec          `yaml:"helmDefaults,omitempty"`
	Helmfiles           []SubHelmfileSpec `yaml:"helmfiles,omitempty"`
	DeprecatedContext   string            `yaml:"context,omitempty"`
	DeprecatedReleases  []ReleaseSpec     `yaml:"charts,omitempty"`
	OverrideKubeContext string            `yaml:"kubeContext,omitempty"`
	OverrideNamespace   string            `yaml:"namespace,omitempty"`
	OverrideChart       string            `yaml:"chart,omitempty"`
	Repositories        []RepositorySpec  `yaml:"repositories,omitempty"`
	CommonLabels        map[string]string `yaml:"commonLabels,omitempty"`
	Releases            []ReleaseSpec     `yaml:"releases,omitempty"`
	Selectors           []string          `yaml:"-"`

	// Capabilities.APIVersions
	ApiVersions []string `yaml:"apiVersions,omitempty"`

	// Capabilities.KubeVersion
	KubeVersion string `yaml:"kubeVersion,omitempty"`

	// Hooks is a list of extension points paired with operations, that are executed in specific points of the lifecycle of releases defined in helmfile
	Hooks []event.Hook `yaml:"hooks,omitempty"`

	Templates map[string]TemplateSpec `yaml:"templates"`

	Env environment.Environment `yaml:"-"`

	// If set to "Error", return an error when a subhelmfile points to a
	// non-existent path. The default behavior is to print a warning. Note the
	// differing default compared to other MissingFileHandlers.
	MissingFileHandler string `yaml:"missingFileHandler,omitempty"`
}

// HelmState structure for the helmfile
type HelmState struct {
	basePath string
	FilePath string

	ReleaseSetSpec `yaml:",inline"`

	logger  *zap.SugaredLogger
	fs      *filesystem.FileSystem
	tempDir func(string, string) (string, error)

	valsRuntime vals.Evaluator

	// RenderedValues is the helmfile-wide values that is `.Values`
	// which is accessible from within the whole helmfile go template.
	// Note that this is usually computed by DesiredStateLoader from ReleaseSetSpec.Env
	RenderedValues map[string]interface{}
}

// SubHelmfileSpec defines the subhelmfile path and options
type SubHelmfileSpec struct {
	//path or glob pattern for the sub helmfiles
	Path string `yaml:"path,omitempty"`
	//chosen selectors for the sub helmfiles
	Selectors []string `yaml:"selectors,omitempty"`
	//do the sub helmfiles inherits from parent selectors
	SelectorsInherited bool `yaml:"selectorsInherited,omitempty"`

	Environment SubhelmfileEnvironmentSpec
}

// SubhelmfileEnvironmentSpec is the environment spec for a subhelmfile
type SubhelmfileEnvironmentSpec struct {
	OverrideValues []interface{} `yaml:"values,omitempty"`
}

// HelmSpec to defines helmDefault values
type HelmSpec struct {
	KubeContext     string   `yaml:"kubeContext,omitempty"`
	TillerNamespace string   `yaml:"tillerNamespace,omitempty"`
	Tillerless      bool     `yaml:"tillerless"`
	Args            []string `yaml:"args,omitempty"`
	Verify          bool     `yaml:"verify"`
	// Devel, when set to true, use development versions, too. Equivalent to version '>0.0.0-0'
	Devel bool `yaml:"devel"`
	// Wait, if set to true, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful
	Wait bool `yaml:"wait"`
	// WaitForJobs, if set and --wait enabled, will wait until all Jobs have been completed before marking the release as successful. It will wait for as long as --timeout
	WaitForJobs bool `yaml:"waitForJobs"`
	// Timeout is the time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks, and waits on pod/pvc/svc/deployment readiness) (default 300)
	Timeout int `yaml:"timeout"`
	// RecreatePods, when set to true, instruct helmfile to perform pods restart for the resource if applicable
	RecreatePods bool `yaml:"recreatePods"`
	// Force, when set to true, forces resource update through delete/recreate if needed
	Force bool `yaml:"force"`
	// Atomic, when set to true, restore previous state in case of a failed install/upgrade attempt
	Atomic bool `yaml:"atomic"`
	// CleanupOnFail, when set to true, the --cleanup-on-fail helm flag is passed to the upgrade command
	CleanupOnFail bool `yaml:"cleanupOnFail,omitempty"`
	// HistoryMax, limit the maximum number of revisions saved per release. Use 0 for no limit (default 10)
	HistoryMax *int `yaml:"historyMax,omitempty"`
	// CreateNamespace, when set to true (default), --create-namespace is passed to helm3 on install/upgrade (ignored for helm2)
	CreateNamespace *bool `yaml:"createNamespace,omitempty"`
	// SkipDeps disables running `helm dependency up` and `helm dependency build` on this release's chart.
	// This is relevant only when your release uses a local chart or a directory containing K8s manifests or a Kustomization
	// as a Helm chart.
	SkipDeps bool `yaml:"skipDeps"`

	TLS                      bool   `yaml:"tls"`
	TLSCACert                string `yaml:"tlsCACert,omitempty"`
	TLSKey                   string `yaml:"tlsKey,omitempty"`
	TLSCert                  string `yaml:"tlsCert,omitempty"`
	DisableValidation        *bool  `yaml:"disableValidation,omitempty"`
	DisableOpenAPIValidation *bool  `yaml:"disableOpenAPIValidation,omitempty"`
}

// RepositorySpec that defines values for a helm repo
type RepositorySpec struct {
	Name            string `yaml:"name,omitempty"`
	URL             string `yaml:"url,omitempty"`
	CaFile          string `yaml:"caFile,omitempty"`
	CertFile        string `yaml:"certFile,omitempty"`
	KeyFile         string `yaml:"keyFile,omitempty"`
	Username        string `yaml:"username,omitempty"`
	Password        string `yaml:"password,omitempty"`
	Managed         string `yaml:"managed,omitempty"`
	OCI             bool   `yaml:"oci,omitempty"`
	PassCredentials string `yaml:"passCredentials,omitempty"`
	SkipTLSVerify   string `yaml:"skipTLSVerify,omitempty"`
}

// ReleaseSpec defines the structure of a helm release
type ReleaseSpec struct {
	// Chart is the name of the chart being installed to create this release
	Chart string `yaml:"chart,omitempty"`

	// ChartPath is the downloaded and modified version of the remote Chart specified by the Chart field.
	// This field is empty when the release is going to use the remote chart as-is, without any modifications(e.g. chartify).
	ChartPath string `yaml:"chartPath,omitempty"`

	// Directory is an alias to Chart which may be of more fit when you want to use a local/remote directory containing
	// K8s manifests or Kustomization as a chart
	Directory string `yaml:"directory,omitempty"`
	// Version is the semver version or version constraint for the chart
	Version string `yaml:"version,omitempty"`
	// Verify enables signature verification on fetched chart.
	// Beware some (or many?) chart repositories and charts don't seem to support it.
	Verify *bool `yaml:"verify,omitempty"`
	// Devel, when set to true, use development versions, too. Equivalent to version '>0.0.0-0'
	Devel *bool `yaml:"devel,omitempty"`
	// Wait, if set to true, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful
	Wait *bool `yaml:"wait,omitempty"`
	// WaitForJobs, if set and --wait enabled, will wait until all Jobs have been completed before marking the release as successful. It will wait for as long as --timeout
	WaitForJobs *bool `yaml:"waitForJobs,omitempty"`
	// Timeout is the time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks, and waits on pod/pvc/svc/deployment readiness) (default 300)
	Timeout *int `yaml:"timeout,omitempty"`
	// RecreatePods, when set to true, instruct helmfile to perform pods restart for the resource if applicable
	RecreatePods *bool `yaml:"recreatePods,omitempty"`
	// Force, when set to true, forces resource update through delete/recreate if needed
	Force *bool `yaml:"force,omitempty"`
	// Installed, when set to true, `delete --purge` the release
	Installed *bool `yaml:"installed,omitempty"`
	// Atomic, when set to true, restore previous state in case of a failed install/upgrade attempt
	Atomic *bool `yaml:"atomic,omitempty"`
	// CleanupOnFail, when set to true, the --cleanup-on-fail helm flag is passed to the upgrade command
	CleanupOnFail *bool `yaml:"cleanupOnFail,omitempty"`
	// HistoryMax, limit the maximum number of revisions saved per release. Use 0 for no limit (default 10)
	HistoryMax *int `yaml:"historyMax,omitempty"`
	// Condition, when set, evaluate the mapping specified in this string to a boolean which decides whether or not to process the release
	Condition string `yaml:"condition,omitempty"`
	// CreateNamespace, when set to true (default), --create-namespace is passed to helm3 on install (ignored for helm2)
	CreateNamespace *bool `yaml:"createNamespace,omitempty"`

	// DisableOpenAPIValidation is rarely used to bypass OpenAPI validations only that is used for e.g.
	// work-around against broken CRs
	// See also:
	// - https://github.com/helm/helm/pull/6819
	// - https://github.com/roboll/helmfile/issues/1167
	DisableOpenAPIValidation *bool `yaml:"disableOpenAPIValidation,omitempty"`

	// DisableValidation is rarely used to bypass the whole validation of manifests against the Kubernetes cluster
	// so that `helm diff` can be run containing a chart that installs both CRD and CRs on first install.
	// FYI, such diff without `--disable-validation` fails on first install because the K8s cluster doesn't have CRDs registered yet.
	DisableValidation *bool `yaml:"disableValidation,omitempty"`

	// DisableValidationOnInstall disables the K8s API validation while running helm-diff on the release being newly installed on helmfile-apply.
	// It is useful when any release contains custom resources for CRDs that is not yet installed onto the cluster.
	DisableValidationOnInstall *bool `yaml:"disableValidationOnInstall,omitempty"`

	// MissingFileHandler is set to either "Error" or "Warn". "Error" instructs helmfile to fail when unable to find a values or secrets file. When "Warn", it prints the file and continues.
	// The default value for MissingFileHandler is "Error".
	MissingFileHandler *string `yaml:"missingFileHandler,omitempty"`
	// Needs is the [TILLER_NS/][NS/]NAME representations of releases that this release depends on.
	Needs []string `yaml:"needs,omitempty"`

	// Hooks is a list of extension points paired with operations, that are executed in specific points of the lifecycle of releases defined in helmfile
	Hooks []event.Hook `yaml:"hooks,omitempty"`

	// Name is the name of this release
	Name      string            `yaml:"name,omitempty"`
	Namespace string            `yaml:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
	Values    []interface{}     `yaml:"values,omitempty"`
	Secrets   []interface{}     `yaml:"secrets,omitempty"`
	SetValues []SetValue        `yaml:"set,omitempty"`

	ValuesTemplate    []interface{} `yaml:"valuesTemplate,omitempty"`
	SetValuesTemplate []SetValue    `yaml:"setTemplate,omitempty"`

	// Capabilities.APIVersions
	ApiVersions []string `yaml:"apiVersions,omitempty"`

	// Capabilities.KubeVersion
	KubeVersion string `yaml:"kubeVersion,omitempty"`

	// The 'env' section is not really necessary any longer, as 'set' would now provide the same functionality
	EnvValues []SetValue `yaml:"env,omitempty"`

	ValuesPathPrefix string `yaml:"valuesPathPrefix,omitempty"`

	TillerNamespace string `yaml:"tillerNamespace,omitempty"`
	Tillerless      *bool  `yaml:"tillerless,omitempty"`

	KubeContext string `yaml:"kubeContext,omitempty"`

	TLS       *bool  `yaml:"tls,omitempty"`
	TLSCACert string `yaml:"tlsCACert,omitempty"`
	TLSKey    string `yaml:"tlsKey,omitempty"`
	TLSCert   string `yaml:"tlsCert,omitempty"`

	// These values are used in templating
	TillerlessTemplate *string `yaml:"tillerlessTemplate,omitempty"`
	VerifyTemplate     *string `yaml:"verifyTemplate,omitempty"`
	WaitTemplate       *string `yaml:"waitTemplate,omitempty"`
	InstalledTemplate  *string `yaml:"installedTemplate,omitempty"`

	// These settings requires helm-x integration to work
	Dependencies          []Dependency  `yaml:"dependencies,omitempty"`
	JSONPatches           []interface{} `yaml:"jsonPatches,omitempty"`
	StrategicMergePatches []interface{} `yaml:"strategicMergePatches,omitempty"`

	// Transformers is the list of Kustomize transformers
	//
	// Each item can be a path to a YAML or go template file, or an embedded transformer declaration as a YAML hash.
	// It's often used to add common labels and annotations to your resources.
	// See https://github.com/kubernetes-sigs/kustomize/blob/master/examples/configureBuiltinPlugin.md#configuring-the-builtin-plugins-instead for more information.
	Transformers []interface{} `yaml:"transformers,omitempty"`
	Adopt        []string      `yaml:"adopt,omitempty"`

	//version of the chart that has really been installed cause desired version may be fuzzy (~2.0.0)
	installedVersion string

	// ForceGoGetter forces the use of go-getter for fetching remote directory as maniefsts/chart/kustomization
	// by parsing the url from `chart` field of the release.
	// This is handy when getting the go-getter url parsing error when it doesn't work as expected.
	// Without this, any error in url parsing result in silently falling-back to normal process of treating `chart:` as the regular
	// helm chart name.
	ForceGoGetter bool `yaml:"forceGoGetter,omitempty"`

	// ForceNamespace is an experimental feature to set metadata.namespace in every K8s resource rendered by the chart,
	// regardless of the template, even when it doesn't have `namespace: {{ .Namespace | quote }}`.
	// This is only needed when you can't FIX your chart to have `namespace: {{ .Namespace }}` AND you're using `helmfile template`.
	// In standard use-cases, `Namespace` should be sufficient.
	// Use this only when you know what you want to do!
	ForceNamespace string `yaml:"forceNamespace,omitempty"`

	// SkipDeps disables running `helm dependency up` and `helm dependency build` on this release's chart.
	// This is relevant only when your release uses a local chart or a directory containing K8s manifests or a Kustomization
	// as a Helm chart.
	SkipDeps *bool `yaml:"skipDeps,omitempty"`
}

// ChartPathOrName returns ChartPath if it is non-empty, and returns Chart otherwise.
// This is useful to redirect helm commands like `helm template`, `helm dependency update`, `helm diff`, and `helm upgrade --install` to
// our modified version of the chart, in case the user configured Helmfile to do modify the chart before being passed to Helm.
func (r ReleaseSpec) ChartPathOrName() string {
	if r.ChartPath != "" {
		return r.ChartPath
	}
	return r.Chart
}

type Release struct {
	ReleaseSpec

	Filtered bool
}

// SetValue are the key values to set on a helm release
type SetValue struct {
	Name   string   `yaml:"name,omitempty"`
	Value  string   `yaml:"value,omitempty"`
	File   string   `yaml:"file,omitempty"`
	Values []string `yaml:"values,omitempty"`
}

// AffectedReleases hold the list of released that where updated, deleted, or in error
type AffectedReleases struct {
	Upgraded []*ReleaseSpec
	Deleted  []*ReleaseSpec
	Failed   []*ReleaseSpec
}

// DefaultEnv is the default environment to use for helm commands
const DefaultEnv = "default"

// MissingFileHandlerError is the error returned when a file is missing
const MissingFileHandlerError = "Error"

// MissingFileHandlerInfo is the info returned when a file is missing
const MissingFileHandlerInfo = "Info"

// MissingFileHandlerWarn is the warning returned when a file is missing
const MissingFileHandlerWarn = "Warn"

// MissingFileHandlerDebug is the debug returned when a file is missing
const MissingFileHandlerDebug = "Debug"

func (st *HelmState) ApplyOverrides(spec *ReleaseSpec) {
	if st.OverrideKubeContext != "" {
		spec.KubeContext = st.OverrideKubeContext
	}
	if st.OverrideNamespace != "" {
		spec.Namespace = st.OverrideNamespace
	}

	var needs []string

	// Since the representation differs between needs and id,
	// correct it by prepending Namespace and KubeContext.
	for i := 0; i < len(spec.Needs); i++ {
		n := spec.Needs[i]

		var kubecontext, ns, name string

		components := strings.Split(n, "/")

		name = components[len(components)-1]

		if len(components) > 1 {
			ns = components[len(components)-2]
		} else if spec.TillerNamespace != "" {
			ns = spec.TillerNamespace
		} else {
			ns = spec.Namespace
		}

		if len(components) > 2 {
			// Join all ID components except last 2 (namespace and name) into kubecontext.
			// Should be safe because resource names do not contain slashes.
			kubecontext = strings.Join(components[:len(components)-2], "/")
		} else {
			kubecontext = spec.KubeContext
		}

		var componentsAfterOverride []string

		if kubecontext != "" {
			componentsAfterOverride = append(componentsAfterOverride, kubecontext)
		}

		// This is intentionally `kubecontext != "" || ns != ""`, but "ns != ""
		// To avoid conflating kubecontext=,namespace=foo,name=bar and kubecontext=foo,namespace=,name=bar
		// as they are both `foo/bar`, we explicitly differentiate each with `foo//bar` and `foo/bar`.
		// Note that `foo//bar` is not always a equivalent to `foo/default/bar` as the default namespace is depedent on
		// the user's kubeconfig.
		if kubecontext != "" || ns != "" {
			componentsAfterOverride = append(componentsAfterOverride, ns)
		}

		componentsAfterOverride = append(componentsAfterOverride, name)

		needs = append(needs, strings.Join(componentsAfterOverride, "/"))
	}

	spec.Needs = needs
}

type RepoUpdater interface {
	IsHelm3() bool
	AddRepo(name, repository, cafile, certfile, keyfile, username, password string, managed string, passCredentials string, skipTLSVerify string) error
	UpdateRepo() error
	RegistryLogin(name string, username string, password string) error
}

func (st *HelmState) SyncRepos(helm RepoUpdater, shouldSkip map[string]bool) ([]string, error) {
	var updated []string

	for _, repo := range st.Repositories {
		if shouldSkip[repo.Name] {
			continue
		}
		var err error
		if repo.OCI {
			username, password := gatherOCIUsernamePassword(repo.Name, repo.Username, repo.Password)
			if username != "" && password != "" {
				err = helm.RegistryLogin(repo.URL, username, password)
			}
		} else {
			err = helm.AddRepo(repo.Name, repo.URL, repo.CaFile, repo.CertFile, repo.KeyFile, repo.Username, repo.Password, repo.Managed, repo.PassCredentials, repo.SkipTLSVerify)
		}

		if err != nil {
			return nil, err
		}

		updated = append(updated, repo.Name)
	}

	return updated, nil
}

func gatherOCIUsernamePassword(repoName string, username string, password string) (string, string) {
	var user, pass string

	if username != "" {
		user = username
	} else if u := os.Getenv(fmt.Sprintf("%s_USERNAME", strings.ToUpper(repoName))); u != "" {
		user = u
	}

	if password != "" {
		pass = password
	} else if p := os.Getenv(fmt.Sprintf("%s_PASSWORD", strings.ToUpper(repoName))); p != "" {
		pass = p
	}

	return user, pass
}

type syncResult struct {
	errors []*ReleaseError
}

type syncPrepareResult struct {
	release *ReleaseSpec
	flags   []string
	errors  []*ReleaseError
	files   []string
}

// SyncReleases wrapper for executing helm upgrade on the releases
func (st *HelmState) prepareSyncReleases(helm helmexec.Interface, additionalValues []string, concurrency int, opt ...SyncOpt) ([]syncPrepareResult, []error) {
	opts := &SyncOpts{}
	for _, o := range opt {
		o.Apply(opts)
	}

	releases := []*ReleaseSpec{}
	for i := range st.Releases {
		releases = append(releases, &st.Releases[i])
	}

	numReleases := len(releases)
	jobs := make(chan *ReleaseSpec, numReleases)
	results := make(chan syncPrepareResult, numReleases)

	res := []syncPrepareResult{}
	errs := []error{}

	mut := sync.Mutex{}

	st.scatterGather(
		concurrency,
		numReleases,
		func() {
			for i := 0; i < numReleases; i++ {
				jobs <- releases[i]
			}
			close(jobs)
		},
		func(workerIndex int) {
			for release := range jobs {
				st.ApplyOverrides(release)

				// If `installed: false`, the only potential operation on this release would be uninstalling.
				// We skip generating values files in that case, because for an uninstall with `helm delete`, we don't need to those.
				// The values files are for `helm upgrade -f values.yaml` calls that happens when the release has `installed: true`.
				// This logic addresses:
				// - https://github.com/roboll/helmfile/issues/519
				// - https://github.com/roboll/helmfile/issues/616
				if !release.Desired() {
					results <- syncPrepareResult{release: release, flags: []string{}, errors: []*ReleaseError{}}
					continue
				}

				// TODO We need a long-term fix for this :)
				// See https://github.com/roboll/helmfile/issues/737
				mut.Lock()
				flags, files, flagsErr := st.flagsForUpgrade(helm, release, workerIndex)
				mut.Unlock()
				if flagsErr != nil {
					results <- syncPrepareResult{errors: []*ReleaseError{newReleaseFailedError(release, flagsErr)}, files: files}
					continue
				}

				errs := []*ReleaseError{}
				for _, value := range additionalValues {
					valfile, err := filepath.Abs(value)
					if err != nil {
						errs = append(errs, newReleaseFailedError(release, err))
					}

					ok, err := st.fs.FileExists(valfile)
					if err != nil {
						errs = append(errs, newReleaseFailedError(release, err))
					} else if !ok {
						errs = append(errs, newReleaseFailedError(release, fmt.Errorf("file does not exist: %s", valfile)))
					}
					flags = append(flags, "--values", valfile)
				}

				if opts.Set != nil {
					for _, s := range opts.Set {
						flags = append(flags, "--set", s)
					}
				}

				if opts.SkipCRDs {
					flags = append(flags, "--skip-crds")
				}

				if opts.Wait {
					flags = append(flags, "--wait")
				}

				if opts.WaitForJobs {
					flags = append(flags, "--wait-for-jobs")
				}

				if len(errs) > 0 {
					results <- syncPrepareResult{errors: errs, files: files}
					continue
				}

				results <- syncPrepareResult{release: release, flags: flags, errors: []*ReleaseError{}, files: files}
			}
		},
		func() {
			for i := 0; i < numReleases; {
				r := <-results
				for _, e := range r.errors {
					errs = append(errs, e)
				}
				res = append(res, r)
				i++
			}
		},
	)

	return res, errs
}

func (st *HelmState) isReleaseInstalled(context helmexec.HelmContext, helm helmexec.Interface, release ReleaseSpec) (bool, error) {
	out, err := st.listReleases(context, helm, &release)
	if err != nil {
		return false, err
	} else if out != "" {
		return true, nil
	}
	return false, nil
}

func (st *HelmState) DetectReleasesToBeDeletedForSync(helm helmexec.Interface, releases []ReleaseSpec) ([]ReleaseSpec, error) {
	deleted := []ReleaseSpec{}
	for i := range releases {
		release := releases[i]

		if !release.Desired() {
			installed, err := st.isReleaseInstalled(st.createHelmContext(&release, 0), helm, release)
			if err != nil {
				return nil, err
			}
			if installed {
				// Otherwise `release` messed up(https://github.com/roboll/helmfile/issues/554)
				r := release
				deleted = append(deleted, r)
			}
		}
	}
	return deleted, nil
}

func (st *HelmState) DetectReleasesToBeDeleted(helm helmexec.Interface, releases []ReleaseSpec) ([]ReleaseSpec, error) {
	detected := []ReleaseSpec{}
	for i := range releases {
		release := releases[i]

		installed, err := st.isReleaseInstalled(st.createHelmContext(&release, 0), helm, release)
		if err != nil {
			return nil, err
		} else if installed {
			// Otherwise `release` messed up(https://github.com/roboll/helmfile/issues/554)
			r := release
			detected = append(detected, r)
		}
	}
	return detected, nil
}

type SyncOpts struct {
	Set         []string
	SkipCleanup bool
	SkipCRDs    bool
	Wait        bool
	WaitForJobs bool
}

type SyncOpt interface{ Apply(*SyncOpts) }

func (o *SyncOpts) Apply(opts *SyncOpts) {
	*opts = *o
}

func ReleaseToID(r *ReleaseSpec) string {
	var id string

	kc := r.KubeContext
	if kc != "" {
		id += kc + "/"
	}

	tns := r.TillerNamespace
	ns := r.Namespace

	if tns != "" {
		id += tns + "/"
	} else if ns != "" {
		id += ns + "/"
	}

	if kc != "" {
		if tns == "" && ns == "" {
			// This is intentional to avoid conflating kc=,ns=foo,name=bar and kc=foo,ns=,name=bar.
			// Before https://github.com/roboll/helmfile/pull/1823 they were both `foo/bar` which turned out to break `needs` in many ways.
			//
			// We now explicitly differentiate each with `foo//bar` and `foo/bar`.
			// Note that `foo//bar` is not always a equivalent to `foo/default/bar` as the default namespace is depedent on
			// the user's kubeconfig.
			// That's why we use `foo//bar` even if it looked unintuitive.
			id += "/"
		}
	}

	id += r.Name

	return id
}

// DeleteReleasesForSync deletes releases that are marked for deletion
func (st *HelmState) DeleteReleasesForSync(affectedReleases *AffectedReleases, helm helmexec.Interface, workerLimit int) []error {
	errs := []error{}

	releases := st.Releases

	jobQueue := make(chan *ReleaseSpec, len(releases))
	results := make(chan syncResult, len(releases))
	if workerLimit == 0 {
		workerLimit = len(releases)
	}

	m := new(sync.Mutex)

	st.scatterGather(
		workerLimit,
		len(releases),
		func() {
			for i := 0; i < len(releases); i++ {
				jobQueue <- &releases[i]
			}
			close(jobQueue)
		},
		func(workerIndex int) {
			for release := range jobQueue {
				var relErr *ReleaseError
				context := st.createHelmContext(release, workerIndex)

				if _, err := st.triggerPresyncEvent(release, "sync"); err != nil {
					relErr = newReleaseFailedError(release, err)
				} else {
					var args []string
					if helm.IsHelm3() {
						args = []string{}
						if release.Namespace != "" {
							args = append(args, "--namespace", release.Namespace)
						}
					} else {
						args = []string{"--purge"}
					}
					deletionFlags := st.appendConnectionFlags(args, helm, release)
					m.Lock()
					if _, err := st.triggerReleaseEvent("preuninstall", nil, release, "sync"); err != nil {
						affectedReleases.Failed = append(affectedReleases.Failed, release)
						relErr = newReleaseFailedError(release, err)
					} else if err := helm.DeleteRelease(context, release.Name, deletionFlags...); err != nil {
						affectedReleases.Failed = append(affectedReleases.Failed, release)
						relErr = newReleaseFailedError(release, err)
					} else if _, err := st.triggerReleaseEvent("postuninstall", nil, release, "sync"); err != nil {
						affectedReleases.Failed = append(affectedReleases.Failed, release)
						relErr = newReleaseFailedError(release, err)
					} else {
						affectedReleases.Deleted = append(affectedReleases.Deleted, release)
					}
					m.Unlock()
				}

				if _, err := st.triggerPostsyncEvent(release, relErr, "sync"); err != nil {
					st.logger.Warnf("warn: %v\n", err)
				}

				if _, err := st.TriggerCleanupEvent(release, "sync"); err != nil {
					st.logger.Warnf("warn: %v\n", err)
				}

				if relErr == nil {
					results <- syncResult{}
				} else {
					results <- syncResult{errors: []*ReleaseError{relErr}}
				}
			}
		},
		func() {
			for i := 0; i < len(releases); {
				res := <-results
				if len(res.errors) > 0 {
					for _, e := range res.errors {
						errs = append(errs, e)
					}
				}
				i++
			}
		},
	)
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// SyncReleases wrapper for executing helm upgrade on the releases
func (st *HelmState) SyncReleases(affectedReleases *AffectedReleases, helm helmexec.Interface, additionalValues []string, workerLimit int, opt ...SyncOpt) []error {
	opts := &SyncOpts{}
	for _, o := range opt {
		o.Apply(opts)
	}

	preps, prepErrs := st.prepareSyncReleases(helm, additionalValues, workerLimit, opts)

	if !opts.SkipCleanup {
		defer func() {
			for _, p := range preps {
				st.removeFiles(p.files)
			}
		}()
	}

	if len(prepErrs) > 0 {
		return prepErrs
	}

	errs := []error{}
	jobQueue := make(chan *syncPrepareResult, len(preps))
	results := make(chan syncResult, len(preps))
	if workerLimit == 0 {
		workerLimit = len(preps)
	}

	m := new(sync.Mutex)

	st.scatterGather(
		workerLimit,
		len(preps),
		func() {
			for i := 0; i < len(preps); i++ {
				jobQueue <- &preps[i]
			}
			close(jobQueue)
		},
		func(workerIndex int) {
			for prep := range jobQueue {
				release := prep.release
				flags := prep.flags
				chart := normalizeChart(st.basePath, release.ChartPathOrName())
				var relErr *ReleaseError
				context := st.createHelmContext(release, workerIndex)

				if _, err := st.triggerPresyncEvent(release, "sync"); err != nil {
					relErr = newReleaseFailedError(release, err)
				} else if !release.Desired() {
					installed, err := st.isReleaseInstalled(context, helm, *release)
					if err != nil {
						relErr = newReleaseFailedError(release, err)
					} else if installed {
						var args []string
						if helm.IsHelm3() {
							args = []string{}
						} else {
							args = []string{"--purge"}
						}
						deletionFlags := st.appendConnectionFlags(args, helm, release)
						m.Lock()
						if _, err := st.triggerReleaseEvent("preuninstall", nil, release, "sync"); err != nil {
							affectedReleases.Failed = append(affectedReleases.Failed, release)
							relErr = newReleaseFailedError(release, err)
						} else if err := helm.DeleteRelease(context, release.Name, deletionFlags...); err != nil {
							affectedReleases.Failed = append(affectedReleases.Failed, release)
							relErr = newReleaseFailedError(release, err)
						} else if _, err := st.triggerReleaseEvent("postuninstall", nil, release, "sync"); err != nil {
							affectedReleases.Failed = append(affectedReleases.Failed, release)
							relErr = newReleaseFailedError(release, err)
						} else {
							affectedReleases.Deleted = append(affectedReleases.Deleted, release)
						}
						m.Unlock()
					}
				} else if err := helm.SyncRelease(context, release.Name, chart, flags...); err != nil {
					m.Lock()
					affectedReleases.Failed = append(affectedReleases.Failed, release)
					m.Unlock()
					relErr = newReleaseFailedError(release, err)
				} else {
					m.Lock()
					affectedReleases.Upgraded = append(affectedReleases.Upgraded, release)
					m.Unlock()
					installedVersion, err := st.getDeployedVersion(context, helm, release)
					if err != nil { // err is not really impacting so just log it
						st.logger.Debugf("getting deployed release version failed:%v", err)
					} else {
						release.installedVersion = installedVersion
					}
				}

				if _, err := st.triggerPostsyncEvent(release, relErr, "sync"); err != nil {
					if relErr == nil {
						relErr = newReleaseFailedError(release, err)
					} else {
						st.logger.Warnf("warn: %v\n", err)
					}
				}

				if _, err := st.TriggerCleanupEvent(release, "sync"); err != nil {
					if relErr == nil {
						relErr = newReleaseFailedError(release, err)
					} else {
						st.logger.Warnf("warn: %v\n", err)
					}
				}

				if relErr == nil {
					results <- syncResult{}
				} else {
					results <- syncResult{errors: []*ReleaseError{relErr}}
				}
			}
		},
		func() {
			for i := 0; i < len(preps); {
				res := <-results
				if len(res.errors) > 0 {
					for _, e := range res.errors {
						errs = append(errs, e)
					}
				}
				i++
			}
		},
	)
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (st *HelmState) listReleases(context helmexec.HelmContext, helm helmexec.Interface, release *ReleaseSpec) (string, error) {
	flags := st.connectionFlags(helm, release)
	if helm.IsHelm3() {
		if release.Namespace != "" {
			flags = append(flags, "--namespace", release.Namespace)
		}
		flags = append(flags, "--uninstalling")
	} else {
		flags = append(flags, "--deleting")
	}
	flags = append(flags, "--deployed", "--failed", "--pending")
	return helm.List(context, "^"+release.Name+"$", flags...)
}

func (st *HelmState) getDeployedVersion(context helmexec.HelmContext, helm helmexec.Interface, release *ReleaseSpec) (string, error) {
	//retrieve the version
	if out, err := st.listReleases(context, helm, release); err == nil {
		chartName := filepath.Base(release.Chart)
		//the regexp without escapes : .*\s.*\s.*\s.*\schartName-(.*?)\s
		pat := regexp.MustCompile(".*\\s.*\\s.*\\s.*\\s" + chartName + "-(.*?)\\s")
		versions := pat.FindStringSubmatch(out)
		if len(versions) > 0 {
			return versions[1], nil
		} else {
			//fails to find the version
			return "failed to get version", errors.New("Failed to get the version for:" + chartName)
		}
	} else {
		return "failed to get version", err
	}
}

func releasesNeedCharts(releases []ReleaseSpec) []ReleaseSpec {
	var result []ReleaseSpec

	for _, r := range releases {
		if r.Installed != nil && !*r.Installed {
			continue
		}
		result = append(result, r)
	}

	return result
}

type ChartPrepareOptions struct {
	ForceDownload bool
	SkipRepos     bool
	SkipDeps      bool
	SkipResolve   bool
	SkipCleanup   bool
	// Validate is a helm-3-only option. When it is set to true, it configures chartify to pass --validate to helm-template run by it.
	// It's required when one of your chart relies on Capabilities.APIVersions in a template
	Validate               bool
	IncludeCRDs            *bool
	Wait                   bool
	WaitForJobs            bool
	OutputDir              string
	IncludeTransitiveNeeds bool
	Concurrency            int
}

type chartPrepareResult struct {
	releaseName            string
	releaseNamespace       string
	releaseContext         string
	chartName              string
	chartPath              string
	err                    error
	buildDeps              bool
	chartFetchedByGoGetter bool
}

func (st *HelmState) GetRepositoryAndNameFromChartName(chartName string) (*RepositorySpec, string) {
	chart := strings.Split(chartName, "/")
	if len(chart) == 1 {
		return nil, chartName
	}
	repo := chart[0]
	for _, r := range st.Repositories {
		if r.Name == repo {
			return &r, strings.Join(chart[1:], "/")
		}
	}
	return nil, chartName
}

type PrepareChartKey struct {
	Namespace, Name, KubeContext string
}

// PrepareCharts creates temporary directories of charts.
//
// Each resulting "chart" can be one of the followings:
//
// (1) local chart
// (2) temporary local chart generated from kustomization or manifests
// (3) remote chart
//
// When running `helmfile template` on helm v2, or `helmfile lint` on both helm v2 and v3,
// PrepareCharts will download and untar charts for linting and templating.
//
// Otheriwse, if a chart is not a helm chart, it will call "chartify" to turn it into a chart.
//
// If exists, it will also patch resources by json patches, strategic-merge patches, and injectors.
func (st *HelmState) PrepareCharts(helm helmexec.Interface, dir string, concurrency int, helmfileCommand string, opts ChartPrepareOptions) (map[PrepareChartKey]string, []error) {
	var selected []ReleaseSpec

	if len(st.Selectors) > 0 {
		var err error

		// This and releasesNeedCharts ensures that we run operations like helm-dep-build and prepare-hook calls only on
		// releases that are (1) selected by the selectors and (2) to be installed.
		selected, err = st.GetSelectedReleasesWithOverrides(opts.IncludeTransitiveNeeds)
		if err != nil {
			return nil, []error{err}
		}
	} else {
		selected = st.Releases
	}

	releases := releasesNeedCharts(selected)

	temp := make(map[PrepareChartKey]string, len(releases))

	errs := []error{}

	jobQueue := make(chan *ReleaseSpec, len(releases))
	results := make(chan *chartPrepareResult, len(releases))

	var helm3 bool

	if helm != nil {
		helm3 = helm.IsHelm3()
	}

	if !opts.SkipResolve {
		updated, err := st.ResolveDeps()
		if err != nil {
			return nil, []error{err}
		}
		*st = *updated
	}

	var builds []*chartPrepareResult

	st.scatterGather(
		concurrency,
		len(releases),
		func() {
			for i := 0; i < len(releases); i++ {
				jobQueue <- &releases[i]
			}
			close(jobQueue)
		},
		func(workerIndex int) {
			for release := range jobQueue {
				if st.OverrideChart != "" {
					release.Chart = st.OverrideChart
				}
				// Call user-defined `prepare` hooks to create/modify local charts to be used by
				// the later process.
				//
				// If it wasn't called here, Helmfile can end up an issue like
				// https://github.com/roboll/helmfile/issues/1328
				if _, err := st.triggerPrepareEvent(release, helmfileCommand); err != nil {
					results <- &chartPrepareResult{err: err}
					return
				}

				chartName := release.Chart

				chartPath, err := st.downloadChartWithGoGetter(release)
				if err != nil {
					results <- &chartPrepareResult{err: fmt.Errorf("release %q: %w", release.Name, err)}
					return
				}
				chartFetchedByGoGetter := chartPath != chartName

				if !chartFetchedByGoGetter {
					ociChartPath, err := st.getOCIChart(release, dir, helm)
					if err != nil {
						results <- &chartPrepareResult{err: fmt.Errorf("release %q: %w", release.Name, err)}

						return
					}

					if ociChartPath != nil {
						chartPath = *ociChartPath
					}
				}

				isLocal := st.fs.DirectoryExistsAt(normalizeChart(st.basePath, chartName))

				chartification, clean, err := st.PrepareChartify(helm, release, chartPath, workerIndex)
				if !opts.SkipCleanup {
					// nolint: staticcheck
					defer clean()
				}
				if err != nil {
					results <- &chartPrepareResult{err: err}
					return
				}

				var buildDeps bool

				skipDepsGlobal := opts.SkipDeps
				skipDepsRelease := release.SkipDeps != nil && *release.SkipDeps
				skipDepsDefault := release.SkipDeps == nil && st.HelmDefaults.SkipDeps
				skipDeps := (!isLocal && !chartFetchedByGoGetter) || skipDepsGlobal || skipDepsRelease || skipDepsDefault

				if chartification != nil {
					c := chartify.New(
						chartify.HelmBin(st.DefaultHelmBinary),
						chartify.UseHelm3(helm3),
						chartify.WithLogf(st.logger.Debugf),
					)

					chartifyOpts := chartification.Opts

					if skipDeps {
						chartifyOpts.SkipDeps = true
					}

					includeCRDs := true
					if opts.IncludeCRDs != nil {
						includeCRDs = *opts.IncludeCRDs
					}
					chartifyOpts.IncludeCRDs = includeCRDs

					chartifyOpts.Validate = opts.Validate

					chartifyOpts.KubeVersion = release.KubeVersion
					chartifyOpts.ApiVersions = release.ApiVersions

					out, err := c.Chartify(release.Name, chartPath, chartify.WithChartifyOpts(chartifyOpts))
					if err != nil {
						results <- &chartPrepareResult{err: err}
						return
					} else {
						chartPath = out
					}

					// Skip `helm dep build` and `helm dep up` altogether when the chart is from remote or the dep is
					// explicitly skipped.
					buildDeps = !skipDeps
				} else if normalizedChart := normalizeChart(st.basePath, chartPath); st.fs.DirectoryExistsAt(normalizedChart) {
					// At this point, we are sure that chartPath is a local directory containing either:
					// - A remote chart fetched by go-getter or
					// - A local chart
					//
					// The chart may have Chart.yaml(and requirements.yaml for Helm 2), and optionally Chart.lock/requirements.lock,
					// but no `charts/` directory populated at all, or a subet of chart dependencies are missing in the directory.
					//
					// In such situation, Helm fails with an error like:
					//   Error: found in Chart.yaml, but missing in charts/ directory: cert-manager, prometheus, postgresql, gitlab-runner, grafana, redis
					//
					// (See also https://github.com/roboll/helmfile/issues/1401#issuecomment-670854495)
					//
					// To avoid it, we need to call a `helm dep build` command on the chart.
					// But the command may consistently fail when an outdated Chart.lock exists.
					//
					// (I've mentioned about such case in https://github.com/roboll/helmfile/pull/1400.)
					//
					// Trying to run `helm dep build` on the chart regardless of if it's from local or remote is
					// problematic, as usually the user would have no way to fix the remote chart on their own.
					//
					// Given that, we always run `helm dep build` on the chart here, but tolerate any error caused by it
					// for a remote chart, so that the user can notice/fix the issue in a local chart while
					// a broken remote chart won't completely block their job.
					chartPath = normalizedChart

					buildDeps = !skipDeps
				} else if !opts.ForceDownload {
					// At this point, we are sure that either:
					// 1. It is a local chart and we can use it in later process (helm upgrade/template/lint/etc)
					//    without any modification, or
					// 2. It is a remote chart which can be safely handed over to helm,
					//    because the version of Helm used in this transaction (helm v3 or greater) support downloading
					//    the chart instead, AND we don't need any modification to the chart
					//
					//    Also see HelmState.chartVersionFlags(). For `helmfile template`, it's called before `helm template`
					//    only on helm v3.
					//    For helm 2, we `helm fetch` with the version flags and call `helm template`
					//    WITHOUT the version flags.
				} else {
					pathElems := []string{
						dir,
					}

					if release.TillerNamespace != "" {
						pathElems = append(pathElems, release.TillerNamespace)
					}

					if release.Namespace != "" {
						pathElems = append(pathElems, release.Namespace)
					}

					if release.KubeContext != "" {
						pathElems = append(pathElems, release.KubeContext)
					}

					chartVersion := "latest"
					if release.Version != "" {
						chartVersion = release.Version
					}

					pathElems = append(pathElems, release.Name, chartName, chartVersion)

					chartPath = path.Join(pathElems...)

					// only fetch chart if it is not already fetched
					if _, err := os.Stat(chartPath); os.IsNotExist(err) {
						fetchFlags := st.chartVersionFlags(release)
						fetchFlags = append(fetchFlags, "--untar", "--untardir", chartPath)
						if err := helm.Fetch(chartName, fetchFlags...); err != nil {
							results <- &chartPrepareResult{err: err}
							return
						}
					}

					// Set chartPath to be the path containing Chart.yaml, if found
					fullChartPath, err := findChartDirectory(chartPath)
					if err == nil {
						chartPath = filepath.Dir(fullChartPath)
					}
				}

				results <- &chartPrepareResult{
					releaseName:            release.Name,
					chartName:              chartName,
					releaseNamespace:       release.Namespace,
					releaseContext:         release.KubeContext,
					chartPath:              chartPath,
					buildDeps:              buildDeps,
					chartFetchedByGoGetter: chartFetchedByGoGetter,
				}
			}
		},
		func() {
			for i := 0; i < len(releases); i++ {
				downloadRes := <-results

				if downloadRes.err != nil {
					errs = append(errs, downloadRes.err)

					return
				}
				temp[PrepareChartKey{
					Namespace:   downloadRes.releaseNamespace,
					KubeContext: downloadRes.releaseContext,
					Name:        downloadRes.releaseName,
				}] = downloadRes.chartPath

				if downloadRes.buildDeps {
					builds = append(builds, downloadRes)
				}
			}
		},
	)

	if len(errs) > 0 {
		return nil, errs
	}

	if len(builds) > 0 {
		if err := st.runHelmDepBuilds(helm, concurrency, builds); err != nil {
			return nil, []error{err}
		}
	}

	return temp, nil
}

// nolint: unparam
func (st *HelmState) runHelmDepBuilds(helm helmexec.Interface, concurrency int, builds []*chartPrepareResult) error {
	// NOTES:
	// 1. `helm dep build` fails when it was run concurrency on the same chart.
	//    To avoid that, we run `helm dep build` only once per each local chart.
	//
	//    See https://github.com/roboll/helmfile/issues/1438
	// 2. Even if it isn't on the same local chart, `helm dep build` intermittently fails when run concurrentl
	//    So we shouldn't use goroutines like we do for other helm operations here.
	//
	//    See https://github.com/roboll/helmfile/issues/1521
	for _, r := range builds {
		if err := helm.BuildDeps(r.releaseName, r.chartPath); err != nil {
			if r.chartFetchedByGoGetter {
				diagnostic := fmt.Sprintf(
					"WARN: `helm dep build` failed. While processing release %q, Helmfile observed that remote chart %q fetched by go-getter is seemingly broken. "+
						"One of well-known causes of this is that the chart has outdated Chart.lock, which needs the chart maintainer needs to run `helm dep up`. "+
						"Helmfile is tolerating the error to avoid blocking you until the remote chart gets fixed. "+
						"But this may result in any failure later if the chart is broken badly. FYI, the tolerated error was: %v",
					r.releaseName,
					r.chartName,
					err.Error(),
				)

				st.logger.Warn(diagnostic)

				continue
			}

			return fmt.Errorf("building dependencies of local chart: %w", err)
		}
	}

	return nil
}

type TemplateOpts struct {
	Set               []string
	SkipCleanup       bool
	OutputDirTemplate string
	IncludeCRDs       bool
	SkipTests         bool
}

type TemplateOpt interface{ Apply(*TemplateOpts) }

func (o *TemplateOpts) Apply(opts *TemplateOpts) {
	*opts = *o
}

// TemplateReleases wrapper for executing helm template on the releases
func (st *HelmState) TemplateReleases(helm helmexec.Interface, outputDir string, additionalValues []string, args []string, workerLimit int,
	validate bool, opt ...TemplateOpt) []error {
	opts := &TemplateOpts{}
	for _, o := range opt {
		o.Apply(opts)
	}

	errs := []error{}

	for i := range st.Releases {
		release := &st.Releases[i]

		if !release.Desired() {
			continue
		}

		st.ApplyOverrides(release)

		flags, files, err := st.flagsForTemplate(helm, release, 0)

		if !opts.SkipCleanup {
			defer st.removeFiles(files)
		}

		if err != nil {
			errs = append(errs, err)
		}

		for _, value := range additionalValues {
			valfile, err := filepath.Abs(value)
			if err != nil {
				errs = append(errs, err)
			}

			if _, err := os.Stat(valfile); os.IsNotExist(err) {
				errs = append(errs, err)
			}
			flags = append(flags, "--values", valfile)
		}

		if opts.Set != nil {
			for _, s := range opts.Set {
				flags = append(flags, "--set", s)
			}
		}

		if len(outputDir) > 0 || len(opts.OutputDirTemplate) > 0 {
			releaseOutputDir, err := st.GenerateOutputDir(outputDir, release, opts.OutputDirTemplate)
			if err != nil {
				errs = append(errs, err)
			}

			flags = append(flags, "--output-dir", releaseOutputDir)
			st.logger.Debugf("Generating templates to : %s\n", releaseOutputDir)
			err = os.MkdirAll(releaseOutputDir, 0755)
			if err != nil {
				errs = append(errs, err)
			}
		}

		if validate {
			flags = append(flags, "--validate")
		}

		if opts.IncludeCRDs {
			flags = append(flags, "--include-crds")
		}

		if opts.SkipTests {
			flags = append(flags, "--skip-tests")
		}

		if len(errs) == 0 {
			if err := helm.TemplateRelease(release.Name, release.ChartPathOrName(), flags...); err != nil {
				errs = append(errs, err)
			}
		}

		if _, err := st.TriggerCleanupEvent(release, "template"); err != nil {
			st.logger.Warnf("warn: %v\n", err)
		}
	}

	if len(errs) != 0 {
		return errs
	}

	return nil
}

type WriteValuesOpts struct {
	Set                []string
	OutputFileTemplate string
	SkipCleanup        bool
}

type WriteValuesOpt interface{ Apply(*WriteValuesOpts) }

func (o *WriteValuesOpts) Apply(opts *WriteValuesOpts) {
	*opts = *o
}

// WriteReleasesValues writes values files for releases
func (st *HelmState) WriteReleasesValues(helm helmexec.Interface, additionalValues []string, opt ...WriteValuesOpt) []error {
	opts := &WriteValuesOpts{}
	for _, o := range opt {
		o.Apply(opts)
	}

	for i := range st.Releases {
		release := &st.Releases[i]

		if !release.Desired() {
			continue
		}

		st.ApplyOverrides(release)

		generatedFiles, err := st.generateValuesFiles(helm, release, i)
		if err != nil {
			return []error{err}
		}

		if !opts.SkipCleanup {
			defer st.removeFiles(generatedFiles)
		}

		for _, value := range additionalValues {
			valfile, err := filepath.Abs(value)
			if err != nil {
				return []error{err}
			}

			if _, err := os.Stat(valfile); os.IsNotExist(err) {
				return []error{err}
			}
		}

		outputValuesFile, err := st.GenerateOutputFilePath(release, opts.OutputFileTemplate)
		if err != nil {
			return []error{err}
		}

		if err := os.MkdirAll(filepath.Dir(outputValuesFile), 0755); err != nil {
			return []error{err}
		}

		st.logger.Infof("Writing values file %s", outputValuesFile)

		merged := map[string]interface{}{}

		for _, f := range append(generatedFiles, additionalValues...) {
			src := map[string]interface{}{}

			srcBytes, err := st.fs.ReadFile(f)
			if err != nil {
				return []error{fmt.Errorf("reading %s: %w", f, err)}
			}

			if err := yaml.Unmarshal(srcBytes, &src); err != nil {
				return []error{fmt.Errorf("unmarshalling yaml %s: %w", f, err)}
			}

			if err := mergo.Merge(&merged, &src, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue); err != nil {
				return []error{fmt.Errorf("merging %s: %w", f, err)}
			}
		}

		var buf bytes.Buffer

		y := yaml.NewEncoder(&buf)
		if err := y.Encode(merged); err != nil {
			return []error{err}
		}

		if err := os.WriteFile(outputValuesFile, buf.Bytes(), 0644); err != nil {
			return []error{fmt.Errorf("writing values file %s: %w", outputValuesFile, err)}
		}

		if _, err := st.TriggerCleanupEvent(release, "write-values"); err != nil {
			st.logger.Warnf("warn: %v\n", err)
		}
	}

	return nil
}

type LintOpts struct {
	Set         []string
	SkipCleanup bool
}

type LintOpt interface{ Apply(*LintOpts) }

func (o *LintOpts) Apply(opts *LintOpts) {
	*opts = *o
}

// LintReleases wrapper for executing helm lint on the releases
func (st *HelmState) LintReleases(helm helmexec.Interface, additionalValues []string, args []string, workerLimit int, opt ...LintOpt) []error {
	opts := &LintOpts{}
	for _, o := range opt {
		o.Apply(opts)
	}

	// Reset the extra args if already set, not to break `helm fetch` by adding the args intended for `lint`
	helm.SetExtraArgs()

	errs := []error{}

	if len(args) > 0 {
		helm.SetExtraArgs(args...)
	}

	for i := range st.Releases {
		release := st.Releases[i]

		if !release.Desired() {
			continue
		}

		flags, files, err := st.flagsForLint(helm, &release, 0)

		if !opts.SkipCleanup {
			defer st.removeFiles(files)
		}

		if err != nil {
			errs = append(errs, err)
		}
		for _, value := range additionalValues {
			valfile, err := filepath.Abs(value)
			if err != nil {
				errs = append(errs, err)
			}

			if _, err := os.Stat(valfile); os.IsNotExist(err) {
				errs = append(errs, err)
			}
			flags = append(flags, "--values", valfile)
		}

		if opts.Set != nil {
			for _, s := range opts.Set {
				flags = append(flags, "--set", s)
			}
		}

		if len(errs) == 0 {
			if err := helm.Lint(release.Name, release.ChartPathOrName(), flags...); err != nil {
				errs = append(errs, err)
			}
		}

		if _, err := st.TriggerCleanupEvent(&release, "lint"); err != nil {
			st.logger.Warnf("warn: %v\n", err)
		}
	}

	if len(errs) != 0 {
		return errs
	}

	return nil
}

type diffResult struct {
	release *ReleaseSpec
	err     *ReleaseError
	buf     *bytes.Buffer
}

type diffPrepareResult struct {
	release                 *ReleaseSpec
	flags                   []string
	errors                  []*ReleaseError
	files                   []string
	upgradeDueToSkippedDiff bool
}

func (st *HelmState) prepareDiffReleases(helm helmexec.Interface, additionalValues []string, concurrency int, detailedExitCode bool, includeTests bool, suppress []string, suppressSecrets bool, showSecrets bool, noHooks bool, opt ...DiffOpt) ([]diffPrepareResult, []error) {
	opts := &DiffOpts{}
	for _, o := range opt {
		o.Apply(opts)
	}

	mu := &sync.Mutex{}
	installedReleases := map[string]bool{}

	isInstalled := func(r *ReleaseSpec) bool {
		mu.Lock()
		defer mu.Unlock()

		id := ReleaseToID(r)

		if v, ok := installedReleases[id]; ok {
			return v
		}

		v, err := st.isReleaseInstalled(st.createHelmContext(r, 0), helm, *r)
		if err != nil {
			st.logger.Warnf("confirming if the release is already installed or not: %v", err)
		} else {
			installedReleases[id] = v
		}

		return v
	}

	releases := []*ReleaseSpec{}
	for i := range st.Releases {
		if !st.Releases[i].Desired() {
			continue
		}
		if st.Releases[i].Installed != nil && !*(st.Releases[i].Installed) {
			continue
		}
		releases = append(releases, &st.Releases[i])
	}

	numReleases := len(releases)
	jobs := make(chan *ReleaseSpec, numReleases)
	results := make(chan diffPrepareResult, numReleases)
	resultsMap := map[string]diffPrepareResult{}

	rs := []diffPrepareResult{}
	errs := []error{}

	mut := sync.Mutex{}

	st.scatterGather(
		concurrency,
		numReleases,
		func() {
			for i := 0; i < numReleases; i++ {
				jobs <- releases[i]
			}
			close(jobs)
		},
		func(workerIndex int) {
			for release := range jobs {
				errs := []error{}

				st.ApplyOverrides(release)

				if opts.SkipDiffOnInstall && !isInstalled(release) {
					results <- diffPrepareResult{release: release, upgradeDueToSkippedDiff: true}
					continue
				}

				disableValidation := release.DisableValidationOnInstall != nil && *release.DisableValidationOnInstall && !isInstalled(release)

				// TODO We need a long-term fix for this :)
				// See https://github.com/roboll/helmfile/issues/737
				mut.Lock()
				flags, files, err := st.flagsForDiff(helm, release, disableValidation, workerIndex)
				mut.Unlock()
				if err != nil {
					errs = append(errs, err)
				}

				for _, value := range additionalValues {
					valfile, err := filepath.Abs(value)
					if err != nil {
						errs = append(errs, err)
					}

					if _, err := os.Stat(valfile); os.IsNotExist(err) {
						errs = append(errs, err)
					}
					flags = append(flags, "--values", valfile)
				}

				if detailedExitCode {
					flags = append(flags, "--detailed-exitcode")
				}

				if includeTests {
					flags = append(flags, "--include-tests")
				}

				for _, s := range suppress {
					flags = append(flags, "--suppress", s)
				}

				if suppressSecrets {
					flags = append(flags, "--suppress-secrets")
				}

				if showSecrets {
					flags = append(flags, "--show-secrets")
				}

				if noHooks {
					flags = append(flags, "--no-hooks")
				}

				if opts.NoColor {
					flags = append(flags, "--no-color")
				} else if opts.Color {
					flags = append(flags, "--color")
				}

				if opts.Context > 0 {
					flags = append(flags, "--context", fmt.Sprintf("%d", opts.Context))
				}

				if opts.Output != "" {
					flags = append(flags, "--output", opts.Output)
				}

				if opts.Set != nil {
					for _, s := range opts.Set {
						flags = append(flags, "--set", s)
					}
				}

				if len(errs) > 0 {
					rsErrs := make([]*ReleaseError, len(errs))
					for i, e := range errs {
						rsErrs[i] = newReleaseFailedError(release, e)
					}
					results <- diffPrepareResult{errors: rsErrs, files: files}
				} else {
					results <- diffPrepareResult{release: release, flags: flags, errors: []*ReleaseError{}, files: files}
				}
			}
		},
		func() {
			for i := 0; i < numReleases; i++ {
				res := <-results
				if res.errors != nil && len(res.errors) > 0 {
					for _, e := range res.errors {
						errs = append(errs, e)
					}
				} else if res.release != nil {
					resultsMap[ReleaseToID(res.release)] = res
				}
			}
		},
	)

	for _, r := range releases {
		if p, ok := resultsMap[ReleaseToID(r)]; ok {
			rs = append(rs, p)
		}
	}

	return rs, errs
}

func (st *HelmState) createHelmContext(spec *ReleaseSpec, workerIndex int) helmexec.HelmContext {
	namespace := st.HelmDefaults.TillerNamespace
	if spec.TillerNamespace != "" {
		namespace = spec.TillerNamespace
	}
	tillerless := st.HelmDefaults.Tillerless
	if spec.Tillerless != nil {
		tillerless = *spec.Tillerless
	}
	historyMax := 10
	if st.HelmDefaults.HistoryMax != nil {
		historyMax = *st.HelmDefaults.HistoryMax
	}
	if spec.HistoryMax != nil {
		historyMax = *spec.HistoryMax
	}

	return helmexec.HelmContext{
		Tillerless:      tillerless,
		TillerNamespace: namespace,
		WorkerIndex:     workerIndex,
		HistoryMax:      historyMax,
	}
}

func (st *HelmState) createHelmContextWithWriter(spec *ReleaseSpec, w io.Writer) helmexec.HelmContext {
	ctx := st.createHelmContext(spec, 0)

	ctx.Writer = w

	return ctx
}

type DiffOpts struct {
	Context int
	Output  string
	// Color forces the color output on helm-diff.
	// This takes effect only when NoColor is false.
	Color bool
	// NoColor forces disabling the color output on helm-diff.
	// If this is true, Color has no effect.
	NoColor           bool
	Set               []string
	SkipCleanup       bool
	SkipDiffOnInstall bool
}

func (o *DiffOpts) Apply(opts *DiffOpts) {
	*opts = *o
}

type DiffOpt interface{ Apply(*DiffOpts) }

// DiffReleases wrapper for executing helm diff on the releases
// It returns releases that had any changes, and errors if any.
//
// This function has responsibility to stabilize the order of writes to stdout from multiple concurrent helm-diff runs.
// It's required to use the stdout from helmfile-diff to detect if there was another change(s) between 2 points in time.
// For example, terraform-provider-helmfile runs a helmfile-diff on `terraform plan` and another on `terraform apply`.
// `terraform`, by design, fails when helmfile-diff outputs were not equivalent.
// Stabilized helmfile-diff output rescues that.
func (st *HelmState) DiffReleases(helm helmexec.Interface, additionalValues []string, workerLimit int, detailedExitCode bool, includeTests bool, suppress []string, suppressSecrets, showSecrets, noHooks bool, suppressDiff, triggerCleanupEvents bool, opt ...DiffOpt) ([]ReleaseSpec, []error) {
	opts := &DiffOpts{}
	for _, o := range opt {
		o.Apply(opts)
	}

	preps, prepErrs := st.prepareDiffReleases(helm, additionalValues, workerLimit, detailedExitCode, includeTests, suppress, suppressSecrets, showSecrets, noHooks, opts)

	if !opts.SkipCleanup {
		defer func() {
			for _, p := range preps {
				st.removeFiles(p.files)
			}
		}()
	}

	if len(prepErrs) > 0 {
		return []ReleaseSpec{}, prepErrs
	}

	jobQueue := make(chan *diffPrepareResult, len(preps))
	results := make(chan diffResult, len(preps))

	rs := []ReleaseSpec{}
	outputs := map[string]*bytes.Buffer{}
	errs := []error{}

	// The exit code returned by helm-diff when it detected any changes
	HelmDiffExitCodeChanged := 2

	st.scatterGather(
		workerLimit,
		len(preps),
		func() {
			for i := 0; i < len(preps); i++ {
				jobQueue <- &preps[i]
			}
			close(jobQueue)
		},
		func(workerIndex int) {
			for prep := range jobQueue {
				flags := prep.flags
				release := prep.release
				buf := &bytes.Buffer{}
				if prep.upgradeDueToSkippedDiff {
					results <- diffResult{release, &ReleaseError{ReleaseSpec: release, err: nil, Code: HelmDiffExitCodeChanged}, buf}
				} else if err := helm.DiffRelease(st.createHelmContextWithWriter(release, buf), release.Name, normalizeChart(st.basePath, release.ChartPathOrName()), suppressDiff, flags...); err != nil {
					switch e := err.(type) {
					case helmexec.ExitError:
						// Propagate any non-zero exit status from the external command like `helm` that is failed under the hood
						results <- diffResult{release, &ReleaseError{release, err, e.ExitStatus()}, buf}
					default:
						results <- diffResult{release, &ReleaseError{release, err, 0}, buf}
					}
				} else {
					// diff succeeded, found no changes
					results <- diffResult{release, nil, buf}
				}

				if triggerCleanupEvents {
					if _, err := st.TriggerCleanupEvent(prep.release, "diff"); err != nil {
						st.logger.Warnf("warn: %v\n", err)
					}
				}
			}
		},
		func() {
			for i := 0; i < len(preps); i++ {
				res := <-results
				if res.err != nil {
					errs = append(errs, res.err)
					if res.err.Code == HelmDiffExitCodeChanged {
						rs = append(rs, *res.err.ReleaseSpec)
					}
				}

				outputs[ReleaseToID(res.release)] = res.buf
			}
		},
	)

	for _, p := range preps {
		id := ReleaseToID(p.release)
		if stdout, ok := outputs[id]; ok {
			fmt.Print(stdout.String())
		} else {
			panic(fmt.Sprintf("missing output for release %s", id))
		}
	}

	return rs, errs
}

func (st *HelmState) ReleaseStatuses(helm helmexec.Interface, workerLimit int) []error {
	return st.scatterGatherReleases(helm, workerLimit, func(release ReleaseSpec, workerIndex int) error {
		if !release.Desired() {
			return nil
		}

		st.ApplyOverrides(&release)

		flags := []string{}
		if helm.IsHelm3() && release.Namespace != "" {
			flags = append(flags, "--namespace", release.Namespace)
		}
		flags = st.appendConnectionFlags(flags, helm, &release)

		return helm.ReleaseStatus(st.createHelmContext(&release, workerIndex), release.Name, flags...)
	})
}

// DeleteReleases wrapper for executing helm delete on the releases
func (st *HelmState) DeleteReleases(affectedReleases *AffectedReleases, helm helmexec.Interface, concurrency int, purge bool) []error {
	return st.scatterGatherReleases(helm, concurrency, func(release ReleaseSpec, workerIndex int) error {
		st.ApplyOverrides(&release)

		flags := []string{}
		if purge && !helm.IsHelm3() {
			flags = append(flags, "--purge")
		}
		flags = st.appendConnectionFlags(flags, helm, &release)
		if helm.IsHelm3() && release.Namespace != "" {
			flags = append(flags, "--namespace", release.Namespace)
		}
		context := st.createHelmContext(&release, workerIndex)

		if _, err := st.triggerReleaseEvent("preuninstall", nil, &release, "delete"); err != nil {
			affectedReleases.Failed = append(affectedReleases.Failed, &release)

			return err
		}

		if err := helm.DeleteRelease(context, release.Name, flags...); err != nil {
			affectedReleases.Failed = append(affectedReleases.Failed, &release)
			return err
		}

		if _, err := st.triggerReleaseEvent("postuninstall", nil, &release, "delete"); err != nil {
			affectedReleases.Failed = append(affectedReleases.Failed, &release)
			return err
		}

		affectedReleases.Deleted = append(affectedReleases.Deleted, &release)
		return nil
	})
}

type TestOpts struct {
	Logs bool
}

type TestOption func(*TestOpts)

func Logs(v bool) func(*TestOpts) {
	return func(o *TestOpts) {
		o.Logs = v
	}
}

// TestReleases wrapper for executing helm test on the releases
func (st *HelmState) TestReleases(helm helmexec.Interface, cleanup bool, timeout int, concurrency int, options ...TestOption) []error {
	var opts TestOpts

	for _, o := range options {
		o(&opts)
	}

	return st.scatterGatherReleases(helm, concurrency, func(release ReleaseSpec, workerIndex int) error {
		if !release.Desired() {
			return nil
		}

		flags := []string{}
		if helm.IsHelm3() && release.Namespace != "" {
			flags = append(flags, "--namespace", release.Namespace)
		}
		if cleanup && !helm.IsHelm3() {
			flags = append(flags, "--cleanup")
		}
		if opts.Logs {
			flags = append(flags, "--logs")
		}

		if timeout == EmptyTimeout {
			flags = append(flags, st.timeoutFlags(helm, &release)...)
		} else {
			duration := strconv.Itoa(timeout)
			if helm.IsHelm3() {
				duration += "s"
			}
			flags = append(flags, "--timeout", duration)
		}

		flags = st.appendConnectionFlags(flags, helm, &release)

		return helm.TestRelease(st.createHelmContext(&release, workerIndex), release.Name, flags...)
	})
}

// Clean will remove any generated secrets
func (st *HelmState) Clean() []error {
	return nil
}

func (st *HelmState) GetReleasesWithOverrides() []ReleaseSpec {
	var rs []ReleaseSpec
	for _, r := range st.Releases {
		spec := r
		st.ApplyOverrides(&spec)
		rs = append(rs, spec)
	}
	return rs
}

func (st *HelmState) SelectReleasesWithOverrides(includeTransitiveNeeds bool) ([]Release, error) {
	values := st.Values()
	rs, err := markExcludedReleases(st.GetReleasesWithOverrides(), st.Selectors, st.CommonLabels, values, includeTransitiveNeeds)
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func markExcludedReleases(releases []ReleaseSpec, selectors []string, commonLabels map[string]string, values map[string]interface{}, includeTransitiveNeeds bool) ([]Release, error) {
	var filteredReleases []Release
	filters := []ReleaseFilter{}
	for _, label := range selectors {
		f, err := ParseLabels(label)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	for _, r := range releases {
		if r.Labels == nil {
			r.Labels = map[string]string{}
		}
		// Let the release name, namespace, and chart be used as a tag
		r.Labels["name"] = r.Name
		r.Labels["namespace"] = r.Namespace
		// Strip off just the last portion for the name stable/newrelic would give newrelic
		chartSplit := strings.Split(r.Chart, "/")
		r.Labels["chart"] = chartSplit[len(chartSplit)-1]
		// Merge CommonLabels into release labels
		for k, v := range commonLabels {
			r.Labels[k] = v
		}
		var filterMatch bool
		for _, f := range filters {
			if r.Labels == nil {
				r.Labels = map[string]string{}
			}
			if f.Match(r) {
				filterMatch = true
				break
			}
		}
		var conditionMatch bool
		conditionMatch, err := ConditionEnabled(r, values)
		if err != nil {
			return nil, fmt.Errorf("failed to parse condition in release %s: %w", r.Name, err)
		}
		res := Release{
			ReleaseSpec: r,
			Filtered:    (len(filters) > 0 && !filterMatch) || (!conditionMatch),
		}
		filteredReleases = append(filteredReleases, res)
	}
	if includeTransitiveNeeds {
		unmarkNeedsAndTransitives(filteredReleases, releases)
	}
	return filteredReleases, nil
}

func ConditionEnabled(r ReleaseSpec, values map[string]interface{}) (bool, error) {
	var conditionMatch bool
	if len(r.Condition) == 0 {
		return true, nil
	}
	conditionSplit := strings.Split(r.Condition, ".")
	if len(conditionSplit) != 2 {
		return false, fmt.Errorf("Condition value must be in the form 'foo.enabled' where 'foo' can be modified as necessary")
	}
	if v, ok := values[conditionSplit[0]]; ok {
		if v == nil {
			panic(fmt.Sprintf("environment values field '%s' is nil", conditionSplit[0]))
		}
		if v.(map[string]interface{})["enabled"] == true {
			conditionMatch = true
		}
	} else {
		panic(fmt.Sprintf("environment values does not contain field '%s'", conditionSplit[0]))
	}

	return conditionMatch, nil
}

func unmarkNeedsAndTransitives(filteredReleases []Release, allReleases []ReleaseSpec) {
	needsWithTranstives := collectAllNeedsWithTransitives(filteredReleases, allReleases)
	unmarkReleases(needsWithTranstives, filteredReleases)
}

func collectAllNeedsWithTransitives(filteredReleases []Release, allReleases []ReleaseSpec) map[string]struct{} {
	needsWithTranstives := map[string]struct{}{}
	for _, r := range filteredReleases {
		if !r.Filtered {
			collectNeedsWithTransitives(r.ReleaseSpec, allReleases, needsWithTranstives)
		}
	}
	return needsWithTranstives
}

func unmarkReleases(toUnmark map[string]struct{}, releases []Release) {
	for i, r := range releases {
		if _, ok := toUnmark[ReleaseToID(&r.ReleaseSpec)]; ok {
			releases[i].Filtered = false
		}
	}
}

func collectNeedsWithTransitives(release ReleaseSpec, allReleases []ReleaseSpec, needsWithTranstives map[string]struct{}) {
	for _, id := range release.Needs {
		if _, exists := needsWithTranstives[id]; !exists {
			needsWithTranstives[id] = struct{}{}
			releaseParts := strings.Split(id, "/")
			releaseName := releaseParts[len(releaseParts)-1]
			for _, r := range allReleases {
				if r.Name == releaseName {
					collectNeedsWithTransitives(r, allReleases, needsWithTranstives)
				}
			}
		}
	}
}

func (st *HelmState) GetSelectedReleasesWithOverrides(includeTransitiveNeeds bool) ([]ReleaseSpec, error) {
	filteredReleases, err := st.SelectReleasesWithOverrides(includeTransitiveNeeds)
	if err != nil {
		return nil, err
	}
	var releases []ReleaseSpec
	for _, r := range filteredReleases {
		if !r.Filtered {
			releases = append(releases, r.ReleaseSpec)
		}
	}

	return releases, nil
}

// FilterReleases allows for the execution of helm commands against a subset of the releases in the helmfile.
func (st *HelmState) FilterReleases(includeTransitiveNeeds bool) error {
	releases, err := st.GetSelectedReleasesWithOverrides(includeTransitiveNeeds)
	if err != nil {
		return err
	}
	st.Releases = releases
	return nil
}

func (st *HelmState) TriggerGlobalPrepareEvent(helmfileCommand string) (bool, error) {
	return st.triggerGlobalReleaseEvent("prepare", nil, helmfileCommand)
}

func (st *HelmState) TriggerGlobalCleanupEvent(helmfileCommand string) (bool, error) {
	return st.triggerGlobalReleaseEvent("cleanup", nil, helmfileCommand)
}

func (st *HelmState) triggerGlobalReleaseEvent(evt string, evtErr error, helmfileCmd string) (bool, error) {
	bus := &event.Bus{
		Hooks:         st.Hooks,
		StateFilePath: st.FilePath,
		BasePath:      st.basePath,
		Namespace:     st.OverrideNamespace,
		Chart:         st.OverrideChart,
		Env:           st.Env,
		Logger:        st.logger,
		Fs:            st.fs,
	}
	data := map[string]interface{}{
		"HelmfileCommand": helmfileCmd,
	}
	return bus.Trigger(evt, evtErr, data)
}

func (st *HelmState) triggerPrepareEvent(r *ReleaseSpec, helmfileCommand string) (bool, error) {
	return st.triggerReleaseEvent("prepare", nil, r, helmfileCommand)
}

func (st *HelmState) TriggerCleanupEvent(r *ReleaseSpec, helmfileCommand string) (bool, error) {
	return st.triggerReleaseEvent("cleanup", nil, r, helmfileCommand)
}

func (st *HelmState) triggerPresyncEvent(r *ReleaseSpec, helmfileCommand string) (bool, error) {
	return st.triggerReleaseEvent("presync", nil, r, helmfileCommand)
}

func (st *HelmState) triggerPostsyncEvent(r *ReleaseSpec, evtErr error, helmfileCommand string) (bool, error) {
	return st.triggerReleaseEvent("postsync", evtErr, r, helmfileCommand)
}

func (st *HelmState) TriggerPreapplyEvent(r *ReleaseSpec, helmfileCommand string) (bool, error) {
	return st.triggerReleaseEvent("preapply", nil, r, helmfileCommand)
}

func (st *HelmState) triggerReleaseEvent(evt string, evtErr error, r *ReleaseSpec, helmfileCmd string) (bool, error) {
	bus := &event.Bus{
		Hooks:         r.Hooks,
		StateFilePath: st.FilePath,
		BasePath:      st.basePath,
		Namespace:     st.OverrideNamespace,
		Chart:         st.OverrideChart,
		Env:           st.Env,
		Logger:        st.logger,
		Fs:            st.fs,
	}
	vals := st.Values()
	data := map[string]interface{}{
		"Values":          vals,
		"Release":         r,
		"HelmfileCommand": helmfileCmd,
	}

	return bus.Trigger(evt, evtErr, data)
}

// ResolveDeps returns a copy of this helmfile state with the concrete chart version numbers filled in for remote chart dependencies
func (st *HelmState) ResolveDeps() (*HelmState, error) {
	return st.mergeLockedDependencies()
}

// UpdateDeps wrapper for updating dependencies on the releases
func (st *HelmState) UpdateDeps(helm helmexec.Interface, includeTransitiveNeeds bool) []error {
	var selected []ReleaseSpec

	if len(st.Selectors) > 0 {
		var err error

		// This and releasesNeedCharts ensures that we run operations like helm-dep-build and prepare-hook calls only on
		// releases that are (1) selected by the selectors and (2) to be installed.
		selected, err = st.GetSelectedReleasesWithOverrides(includeTransitiveNeeds)
		if err != nil {
			return []error{err}
		}
	} else {
		selected = st.Releases
	}

	releases := releasesNeedCharts(selected)

	var errs []error

	for _, release := range releases {
		if st.fs.DirectoryExistsAt(release.ChartPathOrName()) {
			if err := helm.UpdateDeps(release.ChartPathOrName()); err != nil {
				errs = append(errs, err)
			}
		} else {
			st.logger.Debugf("skipped updating dependencies for remote chart %s", release.Chart)
		}
	}

	if len(errs) == 0 {
		tempDir := st.tempDir
		if tempDir == nil {
			tempDir = os.MkdirTemp
		}
		_, err := st.updateDependenciesInTempDir(helm, tempDir)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to update deps: %v", err))
		}
	}

	if len(errs) != 0 {
		return errs
	}
	return nil
}

// find "Chart.yaml"
func findChartDirectory(topLevelDir string) (string, error) {
	var files []string
	err := filepath.Walk(topLevelDir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking through %s: %v", path, err)
		}
		if !f.IsDir() {
			r, err := regexp.MatchString("Chart.yaml", f.Name())
			if err == nil && r {
				files = append(files, path)
			}
		}
		return nil
	})
	if err != nil {
		return topLevelDir, err
	}
	// Sort to get the shortest path
	sort.Strings(files)
	if len(files) > 0 {
		first := files[0]
		return first, nil
	}

	return topLevelDir, errors.New("no Chart.yaml found")
}

// appendConnectionFlags append all the helm command-line flags related to K8s API and Tiller connection including the kubecontext
func (st *HelmState) appendConnectionFlags(flags []string, helm helmexec.Interface, release *ReleaseSpec) []string {
	adds := st.connectionFlags(helm, release)
	flags = append(flags, adds...)
	return flags
}

func (st *HelmState) connectionFlags(helm helmexec.Interface, release *ReleaseSpec) []string {
	flags := []string{}
	tillerless := st.HelmDefaults.Tillerless
	if release.Tillerless != nil {
		tillerless = *release.Tillerless
	}
	if !tillerless {
		if !helm.IsHelm3() {
			if release.TillerNamespace != "" {
				flags = append(flags, "--tiller-namespace", release.TillerNamespace)
			} else if st.HelmDefaults.TillerNamespace != "" {
				flags = append(flags, "--tiller-namespace", st.HelmDefaults.TillerNamespace)
			}
		}

		if release.TLS != nil && *release.TLS || release.TLS == nil && st.HelmDefaults.TLS {
			flags = append(flags, "--tls")
		}

		if release.TLSKey != "" {
			flags = append(flags, "--tls-key", release.TLSKey)
		} else if st.HelmDefaults.TLSKey != "" {
			flags = append(flags, "--tls-key", st.HelmDefaults.TLSKey)
		}

		if release.TLSCert != "" {
			flags = append(flags, "--tls-cert", release.TLSCert)
		} else if st.HelmDefaults.TLSCert != "" {
			flags = append(flags, "--tls-cert", st.HelmDefaults.TLSCert)
		}

		if release.TLSCACert != "" {
			flags = append(flags, "--tls-ca-cert", release.TLSCACert)
		} else if st.HelmDefaults.TLSCACert != "" {
			flags = append(flags, "--tls-ca-cert", st.HelmDefaults.TLSCACert)
		}

		if release.KubeContext != "" {
			flags = append(flags, "--kube-context", release.KubeContext)
		} else if st.Environments[st.Env.Name].KubeContext != "" {
			flags = append(flags, "--kube-context", st.Environments[st.Env.Name].KubeContext)
		} else if st.HelmDefaults.KubeContext != "" {
			flags = append(flags, "--kube-context", st.HelmDefaults.KubeContext)
		}
	}

	return flags
}

func (st *HelmState) timeoutFlags(helm helmexec.Interface, release *ReleaseSpec) []string {
	var flags []string

	timeout := st.HelmDefaults.Timeout
	if release.Timeout != nil {
		timeout = *release.Timeout
	}
	if timeout != 0 {
		duration := strconv.Itoa(timeout)
		if helm.IsHelm3() {
			duration += "s"
		}
		flags = append(flags, "--timeout", duration)
	}

	return flags
}

func (st *HelmState) flagsForUpgrade(helm helmexec.Interface, release *ReleaseSpec, workerIndex int) ([]string, []string, error) {
	flags := st.chartVersionFlags(release)

	if release.Verify != nil && *release.Verify || release.Verify == nil && st.HelmDefaults.Verify {
		flags = append(flags, "--verify")
	}

	if release.Wait != nil && *release.Wait || release.Wait == nil && st.HelmDefaults.Wait {
		flags = append(flags, "--wait")
	}

	if release.WaitForJobs != nil && *release.WaitForJobs || release.WaitForJobs == nil && st.HelmDefaults.WaitForJobs {
		flags = append(flags, "--wait-for-jobs")
	}

	flags = append(flags, st.timeoutFlags(helm, release)...)

	if release.Force != nil && *release.Force || release.Force == nil && st.HelmDefaults.Force {
		flags = append(flags, "--force")
	}

	if release.RecreatePods != nil && *release.RecreatePods || release.RecreatePods == nil && st.HelmDefaults.RecreatePods {
		flags = append(flags, "--recreate-pods")
	}

	if release.Atomic != nil && *release.Atomic || release.Atomic == nil && st.HelmDefaults.Atomic {
		flags = append(flags, "--atomic")
	}

	if release.CleanupOnFail != nil && *release.CleanupOnFail || release.CleanupOnFail == nil && st.HelmDefaults.CleanupOnFail {
		flags = append(flags, "--cleanup-on-fail")
	}

	if release.CreateNamespace != nil && *release.CreateNamespace ||
		release.CreateNamespace == nil && (st.HelmDefaults.CreateNamespace == nil || *st.HelmDefaults.CreateNamespace) {
		if helm.IsVersionAtLeast("3.2.0") {
			flags = append(flags, "--create-namespace")
		} else if release.CreateNamespace != nil || st.HelmDefaults.CreateNamespace != nil {
			// createNamespace was set explicitly, but not running supported version of helm - error
			return nil, nil, fmt.Errorf("releases[].createNamespace requires Helm 3.2.0 or greater")
		}
	}

	if release.DisableOpenAPIValidation != nil && *release.DisableOpenAPIValidation ||
		release.DisableOpenAPIValidation == nil && st.HelmDefaults.DisableOpenAPIValidation != nil && *st.HelmDefaults.DisableOpenAPIValidation {
		flags = append(flags, "--disable-openapi-validation")
	}

	flags = st.appendConnectionFlags(flags, helm, release)

	var err error
	flags, err = st.appendHelmXFlags(flags, release)
	if err != nil {
		return nil, nil, err
	}

	common, clean, err := st.namespaceAndValuesFlags(helm, release, workerIndex)
	if err != nil {
		return nil, clean, err
	}
	return append(flags, common...), clean, nil
}

func (st *HelmState) flagsForTemplate(helm helmexec.Interface, release *ReleaseSpec, workerIndex int) ([]string, []string, error) {
	var flags []string

	// `helm template` in helm v2 does not support `--version` flag. So we fetch with the version flag and then template
	// without the flag. See PrepareCharts function to see the Helmfile implementation of chart fetching.
	//
	// `helm template` in helm v3 supports `--version` and it automatically fetches the remote chart to template,
	// so we skip fetching on helmfile-side and let helm fetch it.
	if helm.IsHelm3() {
		flags = st.chartVersionFlags(release)
	}

	var err error
	flags, err = st.appendHelmXFlags(flags, release)
	if err != nil {
		return nil, nil, err
	}

	flags = st.appendApiVersionsFlags(flags, release)

	common, files, err := st.namespaceAndValuesFlags(helm, release, workerIndex)
	if err != nil {
		return nil, files, err
	}
	return append(flags, common...), files, nil
}

func (st *HelmState) flagsForDiff(helm helmexec.Interface, release *ReleaseSpec, disableValidation bool, workerIndex int) ([]string, []string, error) {
	flags := st.chartVersionFlags(release)

	disableOpenAPIValidation := false
	if release.DisableOpenAPIValidation != nil {
		disableOpenAPIValidation = *release.DisableOpenAPIValidation
	} else if st.HelmDefaults.DisableOpenAPIValidation != nil {
		disableOpenAPIValidation = *st.HelmDefaults.DisableOpenAPIValidation
	}

	if disableOpenAPIValidation {
		flags = append(flags, "--disable-openapi-validation")
	}

	if release.DisableValidation != nil {
		disableValidation = *release.DisableValidation
	} else if st.HelmDefaults.DisableValidation != nil {
		disableValidation = *st.HelmDefaults.DisableValidation
	}

	if disableValidation {
		flags = append(flags, "--disable-validation")
	}

	flags = st.appendApiVersionsFlags(flags, release)

	flags = st.appendConnectionFlags(flags, helm, release)

	var err error
	flags, err = st.appendHelmXFlags(flags, release)
	if err != nil {
		return nil, nil, err
	}

	common, files, err := st.namespaceAndValuesFlags(helm, release, workerIndex)
	if err != nil {
		return nil, files, err
	}
	return append(flags, common...), files, nil
}

func (st *HelmState) chartVersionFlags(release *ReleaseSpec) []string {
	flags := []string{}

	if release.Version != "" {
		flags = append(flags, "--version", release.Version)
	}

	if st.isDevelopment(release) {
		flags = append(flags, "--devel")
	}

	return flags
}

func (st *HelmState) appendApiVersionsFlags(flags []string, r *ReleaseSpec) []string {
	if len(r.ApiVersions) != 0 {
		for _, a := range r.ApiVersions {
			flags = append(flags, "--api-versions", a)
		}
	} else {
		for _, a := range st.ApiVersions {
			flags = append(flags, "--api-versions", a)
		}
	}

	if r.KubeVersion != "" {
		flags = append(flags, "--kube-version", r.KubeVersion)
	} else if st.KubeVersion != "" {
		flags = append(flags, "--kube-version", st.KubeVersion)
	}

	return flags
}

func (st *HelmState) isDevelopment(release *ReleaseSpec) bool {
	result := st.HelmDefaults.Devel
	if release.Devel != nil {
		result = *release.Devel
	}

	return result
}

func (st *HelmState) flagsForLint(helm helmexec.Interface, release *ReleaseSpec, workerIndex int) ([]string, []string, error) {
	flags, files, err := st.namespaceAndValuesFlags(helm, release, workerIndex)
	if err != nil {
		return nil, files, err
	}

	flags, err = st.appendHelmXFlags(flags, release)
	if err != nil {
		return nil, files, err
	}

	return flags, files, nil
}

func (st *HelmState) newReleaseTemplateData(release *ReleaseSpec) releaseTemplateData {
	vals := st.Values()
	templateData := st.createReleaseTemplateData(release, vals)

	return templateData
}

func (st *HelmState) newReleaseTemplateFuncMap(dir string) template.FuncMap {
	r := tmpl.NewFileRenderer(st.fs, dir, nil)

	return r.Context.CreateFuncMap()
}

func (st *HelmState) RenderReleaseValuesFileToBytes(release *ReleaseSpec, path string) ([]byte, error) {
	templateData := st.newReleaseTemplateData(release)

	r := tmpl.NewFileRenderer(st.fs, filepath.Dir(path), templateData)
	rawBytes, err := r.RenderToBytes(path)
	if err != nil {
		return nil, err
	}

	// If 'ref+.*' exists in file, run vals against the file
	match, err := regexp.Match("ref\\+.*", rawBytes)
	if err != nil {
		return nil, err
	}

	if match {
		var rawYaml map[string]interface{}

		if err := yaml.Unmarshal(rawBytes, &rawYaml); err != nil {
			return nil, err
		}

		parsedYaml, err := st.valsRuntime.Eval(rawYaml)
		if err != nil {
			return nil, err
		}

		return maputil.YamlMarshal(parsedYaml)
	}

	return rawBytes, nil
}

func (st *HelmState) storage() *Storage {
	return &Storage{
		FilePath: st.FilePath,
		basePath: st.basePath,
		logger:   st.logger,
		fs:       st.fs,
	}
}

func (st *HelmState) ExpandedHelmfiles() ([]SubHelmfileSpec, error) {
	helmfiles := []SubHelmfileSpec{}
	for _, hf := range st.Helmfiles {
		if remote.IsRemote(hf.Path) {
			helmfiles = append(helmfiles, hf)
			continue
		}

		matches, err := st.storage().ExpandPaths(hf.Path)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			err := fmt.Errorf("no matches for path: %s", hf.Path)
			if st.MissingFileHandler == "Error" {
				return nil, err
			}
			st.logger.Warnf("no matches for path: %s", hf.Path)
			continue
		}
		for _, match := range matches {
			newHelmfile := hf
			newHelmfile.Path = match
			helmfiles = append(helmfiles, newHelmfile)
		}
	}

	return helmfiles, nil
}

func (st *HelmState) removeFiles(files []string) {
	for _, f := range files {
		if err := st.fs.DeleteFile(f); err != nil {
			st.logger.Warnf("Removing %s: %v", err)
		} else {
			st.logger.Debugf("Removed %s", f)
		}
	}
}

func (st *HelmState) generateTemporaryReleaseValuesFiles(release *ReleaseSpec, values []interface{}, missingFileHandler *string) ([]string, error) {
	generatedFiles := []string{}

	for _, value := range values {
		switch typedValue := value.(type) {
		case string:
			paths, skip, err := st.storage().resolveFile(missingFileHandler, "values", typedValue)
			if err != nil {
				return generatedFiles, err
			}
			if skip {
				continue
			}

			if len(paths) > 1 {
				return generatedFiles, fmt.Errorf("glob patterns in release values and secrets is not supported yet. please submit a feature request if necessary")
			}
			path := paths[0]

			yamlBytes, err := st.RenderReleaseValuesFileToBytes(release, path)
			if err != nil {
				return generatedFiles, fmt.Errorf("failed to render values files \"%s\": %v", typedValue, err)
			}

			valfile, err := createTempValuesFile(release, yamlBytes)
			if err != nil {
				return generatedFiles, err
			}
			defer func() {
				_ = valfile.Close()
			}()

			if _, err := valfile.Write(yamlBytes); err != nil {
				return generatedFiles, fmt.Errorf("failed to write %s: %v", valfile.Name(), err)
			}

			st.logger.Debugf("Successfully generated the value file at %s. produced:\n%s", path, string(yamlBytes))

			generatedFiles = append(generatedFiles, valfile.Name())
		case map[interface{}]interface{}, map[string]interface{}:
			valfile, err := createTempValuesFile(release, typedValue)
			if err != nil {
				return generatedFiles, err
			}
			defer func() {
				_ = valfile.Close()
			}()

			encoder := yaml.NewEncoder(valfile)
			defer func() {
				_ = encoder.Close()
			}()

			if err := encoder.Encode(typedValue); err != nil {
				return generatedFiles, err
			}

			generatedFiles = append(generatedFiles, valfile.Name())
		default:
			return generatedFiles, fmt.Errorf("unexpected type of value: value=%v, type=%T", typedValue, typedValue)
		}
	}
	return generatedFiles, nil
}

func (st *HelmState) generateVanillaValuesFiles(release *ReleaseSpec) ([]string, error) {
	values := []interface{}{}
	for _, v := range release.Values {
		switch typedValue := v.(type) {
		case string:
			path := st.storage().normalizePath(release.ValuesPathPrefix + typedValue)
			values = append(values, path)
		default:
			values = append(values, v)
		}
	}

	valuesMapSecretsRendered, err := st.valsRuntime.Eval(map[string]interface{}{"values": values})
	if err != nil {
		return nil, err
	}

	valuesSecretsRendered, ok := valuesMapSecretsRendered["values"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("Failed to render values in %s for release %s: type %T isn't supported", st.FilePath, release.Name, valuesMapSecretsRendered["values"])
	}

	generatedFiles, err := st.generateTemporaryReleaseValuesFiles(release, valuesSecretsRendered, release.MissingFileHandler)
	if err != nil {
		return nil, err
	}

	return generatedFiles, nil
}

func (st *HelmState) generateSecretValuesFiles(helm helmexec.Interface, release *ReleaseSpec, workerIndex int) ([]string, error) {
	var generatedDecryptedFiles []interface{}

	for _, v := range release.Secrets {
		var (
			paths []string
			skip  bool
			err   error
		)

		switch value := v.(type) {
		case string:
			paths, skip, err = st.storage().resolveFile(release.MissingFileHandler, "secrets", release.ValuesPathPrefix+value)
			if err != nil {
				return nil, err
			}
		default:
			bs, err := maputil.YamlMarshal(value)
			if err != nil {
				return nil, err
			}

			path, err := os.CreateTemp(os.TempDir(), "helmfile-embdedded-secrets-*.yaml.enc")
			if err != nil {
				return nil, err
			}
			_ = path.Close()
			defer func() {
				_ = os.Remove(path.Name())
			}()

			if err := os.WriteFile(path.Name(), bs, 0644); err != nil {
				return nil, err
			}

			paths = []string{path.Name()}
		}

		if skip {
			continue
		}

		if len(paths) > 1 {
			return nil, fmt.Errorf("glob patterns in release secret file is not supported yet. please submit a feature request if necessary")
		}
		path := paths[0]

		decryptFlags := st.appendConnectionFlags([]string{}, helm, release)
		valfile, err := helm.DecryptSecret(st.createHelmContext(release, workerIndex), path, decryptFlags...)
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = os.Remove(valfile)
		}()

		generatedDecryptedFiles = append(generatedDecryptedFiles, valfile)
	}

	generatedFiles, err := st.generateTemporaryReleaseValuesFiles(release, generatedDecryptedFiles, release.MissingFileHandler)
	if err != nil {
		return nil, err
	}

	return generatedFiles, nil
}

func (st *HelmState) generateValuesFiles(helm helmexec.Interface, release *ReleaseSpec, workerIndex int) ([]string, error) {
	valuesFiles, err := st.generateVanillaValuesFiles(release)
	if err != nil {
		return nil, err
	}

	secretValuesFiles, err := st.generateSecretValuesFiles(helm, release, workerIndex)
	if err != nil {
		return nil, err
	}

	files := append(valuesFiles, secretValuesFiles...)

	return files, nil
}

func (st *HelmState) namespaceAndValuesFlags(helm helmexec.Interface, release *ReleaseSpec, workerIndex int) ([]string, []string, error) {
	flags := []string{}
	if release.Namespace != "" {
		flags = append(flags, "--namespace", release.Namespace)
	}

	var files []string

	generatedFiles, err := st.generateValuesFiles(helm, release, workerIndex)
	if err != nil {
		return nil, files, err
	}

	files = generatedFiles

	for _, f := range generatedFiles {
		flags = append(flags, "--values", f)
	}

	if len(release.SetValues) > 0 {
		setFlags, err := st.setFlags(release.SetValues)
		if err != nil {
			return nil, files, fmt.Errorf("Failed to render set value entry in %s for release %s: %v", st.FilePath, release.Name, err)
		}

		flags = append(flags, setFlags...)
	}

	/***********
	 * START 'env' section for backwards compatibility
	 ***********/
	// The 'env' section is not really necessary any longer, as 'set' would now provide the same functionality
	if len(release.EnvValues) > 0 {
		val := []string{}
		envValErrs := []string{}
		for _, set := range release.EnvValues {
			value, isSet := os.LookupEnv(set.Value)
			if isSet {
				val = append(val, fmt.Sprintf("%s=%s", escape(set.Name), escape(value)))
			} else {
				errMsg := fmt.Sprintf("\t%s", set.Value)
				envValErrs = append(envValErrs, errMsg)
			}
		}
		if len(envValErrs) != 0 {
			joinedEnvVals := strings.Join(envValErrs, "\n")
			errMsg := fmt.Sprintf("Environment Variables not found. Please make sure they are set and try again:\n%s", joinedEnvVals)
			return nil, files, errors.New(errMsg)
		}
		flags = append(flags, "--set", strings.Join(val, ","))
	}
	/**************
	 * END 'env' section for backwards compatibility
	 **************/

	return flags, files, nil
}

func (st *HelmState) setFlags(setValues []SetValue) ([]string, error) {
	var flags []string

	for _, set := range setValues {
		if set.Value != "" {
			renderedValue, err := renderValsSecrets(st.valsRuntime, set.Value)
			if err != nil {
				return nil, err
			}
			flags = append(flags, "--set", fmt.Sprintf("%s=%s", escape(set.Name), escape(renderedValue[0])))
		} else if set.File != "" {
			flags = append(flags, "--set-file", fmt.Sprintf("%s=%s", escape(set.Name), st.storage().normalizePath(set.File)))
		} else if len(set.Values) > 0 {
			renderedValues, err := renderValsSecrets(st.valsRuntime, set.Values...)
			if err != nil {
				return nil, err
			}
			items := make([]string, len(renderedValues))
			for i, raw := range renderedValues {
				items[i] = escape(raw)
			}
			v := strings.Join(items, ",")
			flags = append(flags, "--set", fmt.Sprintf("%s={%s}", escape(set.Name), v))
		}
	}

	return flags, nil
}

// renderValsSecrets helper function which renders 'ref+.*' secrets
func renderValsSecrets(e vals.Evaluator, input ...string) ([]string, error) {
	output := make([]string, len(input))
	if len(input) > 0 {
		mapRendered, err := e.Eval(map[string]interface{}{"values": input})
		if err != nil {
			return nil, err
		}

		rendered, ok := mapRendered["values"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("type %T isn't supported", mapRendered["values"])
		}

		for i := 0; i < len(rendered); i++ {
			output[i] = fmt.Sprintf("%v", rendered[i])
		}
	}
	return output, nil
}

// DisplayAffectedReleases logs the upgraded, deleted and in error releases
func (ar *AffectedReleases) DisplayAffectedReleases(logger *zap.SugaredLogger) {
	if ar.Upgraded != nil && len(ar.Upgraded) > 0 {
		logger.Info("\nUPDATED RELEASES:")
		tbl, _ := prettytable.NewTable(prettytable.Column{Header: "NAME"},
			prettytable.Column{Header: "CHART", MinWidth: 6},
			prettytable.Column{Header: "VERSION", AlignRight: true},
		)
		tbl.Separator = "   "
		for _, release := range ar.Upgraded {
			err := tbl.AddRow(release.Name, release.Chart, release.installedVersion)
			if err != nil {
				logger.Warn("Could not add row, %v", err)
			}
		}
		logger.Info(tbl.String())
	}
	if ar.Deleted != nil && len(ar.Deleted) > 0 {
		logger.Info("\nDELETED RELEASES:")
		logger.Info("NAME")
		for _, release := range ar.Deleted {
			logger.Info(release.Name)
		}
	}
	if ar.Failed != nil && len(ar.Failed) > 0 {
		logger.Info("\nFAILED RELEASES:")
		logger.Info("NAME")
		for _, release := range ar.Failed {
			logger.Info(release.Name)
		}
	}
}

func escape(value string) string {
	intermediate := strings.ReplaceAll(value, "{", "\\{")
	intermediate = strings.ReplaceAll(intermediate, "}", "\\}")
	return strings.ReplaceAll(intermediate, ",", "\\,")
}

// MarshalYAML will ensure we correctly marshal SubHelmfileSpec structure correctly so it can be unmarshalled at some
// future time
func (p SubHelmfileSpec) MarshalYAML() (interface{}, error) {
	type SubHelmfileSpecTmp struct {
		Path               string        `yaml:"path,omitempty"`
		Selectors          []string      `yaml:"selectors,omitempty"`
		SelectorsInherited bool          `yaml:"selectorsInherited,omitempty"`
		OverrideValues     []interface{} `yaml:"values,omitempty"`
	}
	return &SubHelmfileSpecTmp{
		Path:               p.Path,
		Selectors:          p.Selectors,
		SelectorsInherited: p.SelectorsInherited,
		OverrideValues:     p.Environment.OverrideValues,
	}, nil
}

// UnmarshalYAML will unmarshal the helmfile yaml section and fill the SubHelmfileSpec structure
// this is required to keep allowing string scalar for defining helmfile
func (hf *SubHelmfileSpec) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var tmp interface{}
	if err := unmarshal(&tmp); err != nil {
		return err
	}

	switch i := tmp.(type) {
	case string: // single path definition without sub items, legacy sub helmfile definition
		hf.Path = i
	case map[interface{}]interface{}, map[string]interface{}: // helmfile path with sub section
		var subHelmfileSpecTmp struct {
			Path               string   `yaml:"path"`
			Selectors          []string `yaml:"selectors"`
			SelectorsInherited bool     `yaml:"selectorsInherited"`

			Environment SubhelmfileEnvironmentSpec `yaml:",inline"`
		}
		if err := unmarshal(&subHelmfileSpecTmp); err != nil {
			return err
		}
		hf.Path = subHelmfileSpecTmp.Path
		hf.Selectors = subHelmfileSpecTmp.Selectors
		hf.SelectorsInherited = subHelmfileSpecTmp.SelectorsInherited
		hf.Environment = subHelmfileSpecTmp.Environment
	}
	// since we cannot make sur the "console" string can be red after the "path" we must check we don't have
	// a SubHelmfileSpec with only selector and no path
	if hf.Selectors != nil && hf.Path == "" {
		return fmt.Errorf("found 'selectors' definition without path: %v", hf.Selectors)
	}
	// also exclude SelectorsInherited to true and explicit selectors
	if hf.SelectorsInherited && len(hf.Selectors) > 0 {
		return fmt.Errorf("you cannot use 'SelectorsInherited: true' along with and explicit selector for path: %v", hf.Path)
	}
	return nil
}

func (st *HelmState) GenerateOutputDir(outputDir string, release *ReleaseSpec, outputDirTemplate string) (string, error) {
	// get absolute path of state file to generate a hash
	// use this hash to write helm output in a specific directory by state file and release name
	// ie. in a directory named stateFileName-stateFileHash-releaseName
	stateAbsPath, err := filepath.Abs(st.FilePath)
	if err != nil {
		return stateAbsPath, err
	}

	hasher := sha1.New()
	_, err = io.WriteString(hasher, stateAbsPath)
	if err != nil {
		return "", err
	}

	var stateFileExtension = filepath.Ext(st.FilePath)
	var stateFileName = st.FilePath[0 : len(st.FilePath)-len(stateFileExtension)]

	sha1sum := hex.EncodeToString(hasher.Sum(nil))[:8]

	var sb strings.Builder
	sb.WriteString(stateFileName)
	sb.WriteString("-")
	sb.WriteString(sha1sum)
	sb.WriteString("-")
	sb.WriteString(release.Name)

	if outputDirTemplate == "" {
		outputDirTemplate = filepath.Join("{{ .OutputDir }}", "{{ .State.BaseName }}-{{ .State.AbsPathSHA1 }}-{{ .Release.Name}}")
	}

	t, err := template.New("output-dir").Parse(outputDirTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing output-dir template")
	}

	buf := &bytes.Buffer{}

	type state struct {
		BaseName    string
		Path        string
		AbsPath     string
		AbsPathSHA1 string
	}

	data := struct {
		OutputDir string
		State     state
		Release   *ReleaseSpec
	}{
		OutputDir: outputDir,
		State: state{
			BaseName:    stateFileName,
			Path:        st.FilePath,
			AbsPath:     stateAbsPath,
			AbsPathSHA1: sha1sum,
		},
		Release: release,
	}

	if err := t.Execute(buf, data); err != nil {
		return "", fmt.Errorf("executing output-dir template: %w", err)
	}

	return buf.String(), nil
}

func (st *HelmState) GenerateOutputFilePath(release *ReleaseSpec, outputFileTemplate string) (string, error) {
	// get absolute path of state file to generate a hash
	// use this hash to write helm output in a specific directory by state file and release name
	// ie. in a directory named stateFileName-stateFileHash-releaseName
	stateAbsPath, err := filepath.Abs(st.FilePath)
	if err != nil {
		return stateAbsPath, err
	}

	hasher := sha1.New()
	_, err = io.WriteString(hasher, stateAbsPath)
	if err != nil {
		return "", err
	}

	var stateFileExtension = filepath.Ext(st.FilePath)
	var stateFileName = st.FilePath[0 : len(st.FilePath)-len(stateFileExtension)]

	sha1sum := hex.EncodeToString(hasher.Sum(nil))[:8]

	var sb strings.Builder
	sb.WriteString(stateFileName)
	sb.WriteString("-")
	sb.WriteString(sha1sum)
	sb.WriteString("-")
	sb.WriteString(release.Name)

	if outputFileTemplate == "" {
		outputFileTemplate = filepath.Join("{{ .State.BaseName }}-{{ .State.AbsPathSHA1 }}", "{{ .Release.Name }}.yaml")
	}

	t, err := template.New("output-file").Parse(outputFileTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing output-file template")
	}

	buf := &bytes.Buffer{}

	type state struct {
		BaseName    string
		Path        string
		AbsPath     string
		AbsPathSHA1 string
	}

	data := struct {
		State   state
		Release *ReleaseSpec
	}{
		State: state{
			BaseName:    stateFileName,
			Path:        st.FilePath,
			AbsPath:     stateAbsPath,
			AbsPathSHA1: sha1sum,
		},
		Release: release,
	}

	if err := t.Execute(buf, data); err != nil {
		return "", fmt.Errorf("executing output-file template: %w", err)
	}

	return buf.String(), nil
}

func (st *HelmState) ToYaml() (string, error) {
	if result, err := maputil.YamlMarshal(st); err != nil {
		return "", err
	} else {
		return string(result), nil
	}
}

func (st *HelmState) LoadYAMLForEmbedding(release *ReleaseSpec, entries []interface{}, missingFileHandler *string, pathPrefix string) ([]interface{}, error) {
	var result []interface{}

	for _, v := range entries {
		switch t := v.(type) {
		case string:
			var values map[string]interface{}

			paths, skip, err := st.storage().resolveFile(missingFileHandler, "values", pathPrefix+t)
			if err != nil {
				return nil, err
			}
			if skip {
				continue
			}

			if len(paths) > 1 {
				return nil, fmt.Errorf("glob patterns in release values and secrets is not supported yet. please submit a feature request if necessary")
			}
			yamlOrTemplatePath := paths[0]

			yamlBytes, err := st.RenderReleaseValuesFileToBytes(release, yamlOrTemplatePath)
			if err != nil {
				return nil, fmt.Errorf("failed to render values files \"%s\": %v", t, err)
			}

			if err := yaml.Unmarshal(yamlBytes, &values); err != nil {
				return nil, err
			}

			result = append(result, values)
		default:
			result = append(result, v)
		}
	}

	return result, nil
}

func (st *HelmState) Reverse() {
	for i, j := 0, len(st.Releases)-1; i < j; i, j = i+1, j-1 {
		st.Releases[i], st.Releases[j] = st.Releases[j], st.Releases[i]
	}

	for i, j := 0, len(st.Helmfiles)-1; i < j; i, j = i+1, j-1 {
		st.Helmfiles[i], st.Helmfiles[j] = st.Helmfiles[j], st.Helmfiles[i]
	}
}

func (st *HelmState) getOCIChart(release *ReleaseSpec, tempDir string, helm helmexec.Interface) (*string, error) {
	repo, name := st.GetRepositoryAndNameFromChartName(release.Chart)
	if repo == nil {
		return nil, nil
	}

	if !repo.OCI {
		return nil, nil
	}

	chartVersion := "latest"
	if release.Version != "" {
		chartVersion = release.Version
	}

	qualifiedChartName := fmt.Sprintf("%s/%s:%s", repo.URL, name, chartVersion)

	pathElems := []string{
		tempDir,
	}

	if release.Namespace != "" {
		pathElems = append(pathElems, release.Namespace)
	}

	if release.KubeContext != "" {
		pathElems = append(pathElems, release.KubeContext)
	}

	pathElems = append(pathElems, release.Name, name, chartVersion)

	chartPath := path.Join(pathElems...)

	err := helm.ChartPull(qualifiedChartName, chartPath)
	if err != nil {
		return nil, err
	}
	err = helm.ChartExport(qualifiedChartName, chartPath)
	if err != nil {
		return nil, err
	}

	fullChartPath, err := findChartDirectory(chartPath)
	if err != nil {
		return nil, err
	}

	chartPath = filepath.Dir(fullChartPath)

	return &chartPath, nil
}
