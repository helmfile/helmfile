package state

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"dario.cat/mergo"
	"github.com/Masterminds/semver/v3"
	"github.com/helmfile/chartify"
	"github.com/helmfile/vals"
	"github.com/tatsushid/go-prettytable"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/cli"

	"github.com/helmfile/helmfile/pkg/argparser"
	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/event"
	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/tmpl"
	"github.com/helmfile/helmfile/pkg/yaml"
)

const (
	// EmptyTimeout represents the `--timeout` value passed to helm commands not being specified via helmfile flags.
	// This is used by an interim solution to make the urfave/cli command report to the helmfile internal about that the
	// --timeout flag is missingl
	EmptyTimeout = -1
)

// ReleaseSetSpec is release set spec
type ReleaseSetSpec struct {
	DefaultHelmBinary      string `yaml:"helmBinary,omitempty"`
	DefaultKustomizeBinary string `yaml:"kustomizeBinary,omitempty"`

	// DefaultValues is the default values to be overrode by environment values and command-line overrides
	DefaultValues []any `yaml:"values,omitempty"`

	Environments map[string]EnvironmentSpec `yaml:"environments,omitempty"`

	Bases        []string          `yaml:"bases,omitempty"`
	HelmDefaults HelmSpec          `yaml:"helmDefaults,omitempty"`
	Helmfiles    []SubHelmfileSpec `yaml:"helmfiles,omitempty"`

	// TODO: Remove this function once Helmfile v0.x
	DeprecatedContext  string        `yaml:"context,omitempty"`
	DeprecatedReleases []ReleaseSpec `yaml:"charts,omitempty"`

	OverrideKubeContext string            `yaml:"kubeContext,omitempty"`
	OverrideNamespace   string            `yaml:"namespace,omitempty"`
	OverrideChart       string            `yaml:"chart,omitempty"`
	Repositories        []RepositorySpec  `yaml:"repositories,omitempty"`
	CommonLabels        map[string]string `yaml:"commonLabels,omitempty"`
	Releases            []ReleaseSpec     `yaml:"releases,omitempty"`
	OrginReleases       []ReleaseSpec     `yaml:"-"`
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
	// MissingFileHandlerConfig is composed of various settings for the MissingFileHandler
	MissingFileHandlerConfig MissingFileHandlerConfig `yaml:"missingFileHandlerConfig,omitempty"`

	LockFile string `yaml:"lockFilePath,omitempty"`
}

type MissingFileHandlerConfig struct {
	// IgnoreMissingGitBranch is set to true in order to let the missing file handler
	// treat missing git branch errors like `pathspec 'develop' did not match any file(s) known to git` safe
	// and ignored when the handler is set to Warn or Info.
	IgnoreMissingGitBranch bool `yaml:"ignoreMissingGitBranch,omitempty"`
}

// helmStateAlias is helm state alias
type helmStateAlias HelmState

func (hs *HelmState) UnmarshalYAML(unmarshal func(any) error) error {
	helmStateInfo := make(map[string]any)
	if err := unmarshal(&helmStateInfo); err != nil {
		return err
	}

	return unmarshal((*helmStateAlias)(hs))
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
	RenderedValues map[string]any
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
	OverrideValues []any `yaml:"values,omitempty"`
}

// HelmSpec to defines helmDefault values
type HelmSpec struct {
	KubeContext string   `yaml:"kubeContext,omitempty"`
	Args        []string `yaml:"args,omitempty"`
	DiffArgs    []string `yaml:"diffArgs,omitempty"`
	SyncArgs    []string `yaml:"syncArgs,omitempty"`
	Verify      bool     `yaml:"verify"`
	Keyring     string   `yaml:"keyring,omitempty"`
	// EnableDNS, when set to true, enable DNS lookups when rendering templates
	EnableDNS bool `yaml:"enableDNS"`
	// Propagate '--skip-schema-validation' to helmv3 template and helm install
	SkipSchemaValidation *bool `yaml:"skipSchemaValidation,omitempty"`
	// Devel, when set to true, use development versions, too. Equivalent to version '>0.0.0-0'
	Devel bool `yaml:"devel"`
	// Wait, if set to true, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful
	Wait bool `yaml:"wait"`
	// WaitRetries, if set and --wait enabled, will retry any failed check on resource state, except if HTTP status code < 500 is received, subject to the specified number of retries
	WaitRetries int `yaml:"waitRetries"`
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
	// SkipRefresh disables running `helm dependency up`
	SkipRefresh bool `yaml:"skipRefresh"`
	// on helm upgrade/diff, reuse values currently set in the release and merge them with the ones defined within helmfile
	ReuseValues bool `yaml:"reuseValues"`
	// Propagate '--post-renderer' to helmv3 template and helm install
	PostRenderer *string `yaml:"postRenderer,omitempty"`
	// Propagate '--post-renderer-args' to helmv3 template and helm install
	PostRendererArgs []string `yaml:"postRendererArgs,omitempty"`
	// Cascade '--cascade' to helmv3 delete, available values: background, foreground, or orphan, default: background
	Cascade *string `yaml:"cascade,omitempty"`
	// SuppressOutputLineRegex is a list of regexes to suppress output lines
	SuppressOutputLineRegex []string `yaml:"suppressOutputLineRegex,omitempty"`

	DisableValidation        *bool `yaml:"disableValidation,omitempty"`
	DisableOpenAPIValidation *bool `yaml:"disableOpenAPIValidation,omitempty"`
	// InsecureSkipTLSVerify is true if the TLS verification should be skipped when fetching remote chart
	InsecureSkipTLSVerify bool `yaml:"insecureSkipTLSVerify,omitempty"`
	// PlainHttp is true if the remote charte should be fetched using HTTP and not HTTPS
	PlainHttp bool `yaml:"plainHttp,omitempty"`
	// Wait, if set to true, will wait until all resources are deleted before mark delete command as successful
	DeleteWait bool `yaml:"deleteWait"`
	// Timeout is the time in seconds to wait for helmfile delete command (default 300)
	DeleteTimeout int `yaml:"deleteTimeout"`
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
	RegistryConfig  string `yaml:"registryConfig,omitempty"`
	Managed         string `yaml:"managed,omitempty"`
	OCI             bool   `yaml:"oci,omitempty"`
	Verify          bool   `yaml:"verify,omitempty"`
	Keyring         string `yaml:"keyring,omitempty"`
	PassCredentials bool   `yaml:"passCredentials,omitempty"`
	SkipTLSVerify   bool   `yaml:"skipTLSVerify,omitempty"`
	PlainHttp       bool   `yaml:"plainHttp,omitempty"`
}

type Inherit struct {
	Template string   `yaml:"template,omitempty"`
	Except   []string `yaml:"except,omitempty"`
}

type Inherits []Inherit

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
	Verify  *bool  `yaml:"verify,omitempty"`
	Keyring string `yaml:"keyring,omitempty"`
	// EnableDNS, when set to true, enable DNS lookups when rendering templates
	EnableDNS *bool `yaml:"enableDNS,omitempty"`
	// Devel, when set to true, use development versions, too. Equivalent to version '>0.0.0-0'
	Devel *bool `yaml:"devel,omitempty"`
	// Wait, if set to true, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful
	Wait *bool `yaml:"wait,omitempty"`
	// WaitRetries, if set and --wait enabled, will retry any failed check on resource state, except if HTTP status code < 500 is received, subject to the specified number of retries
	WaitRetries *int `yaml:"waitRetries,omitempty"`
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
	// Needs is the [KUBECONTEXT/][NS/]NAME representations of releases that this release depends on.
	Needs []string `yaml:"needs,omitempty"`

	// Hooks is a list of extension points paired with operations, that are executed in specific points of the lifecycle of releases defined in helmfile
	Hooks []event.Hook `yaml:"hooks,omitempty"`

	// Name is the name of this release
	Name            string            `yaml:"name,omitempty"`
	Namespace       string            `yaml:"namespace,omitempty"`
	Labels          map[string]string `yaml:"labels,omitempty"`
	Values          []any             `yaml:"values,omitempty"`
	Secrets         []any             `yaml:"secrets,omitempty"`
	SetValues       []SetValue        `yaml:"set,omitempty"`
	SetStringValues []SetValue        `yaml:"setString,omitempty"`
	duration        time.Duration

	ValuesTemplate    []any      `yaml:"valuesTemplate,omitempty"`
	SetValuesTemplate []SetValue `yaml:"setTemplate,omitempty"`

	// Capabilities.APIVersions
	ApiVersions []string `yaml:"apiVersions,omitempty"`

	// Capabilities.KubeVersion
	KubeVersion string `yaml:"kubeVersion,omitempty"`

	// The 'env' section is not really necessary any longer, as 'set' would now provide the same functionality
	EnvValues []SetValue `yaml:"env,omitempty"`

	ValuesPathPrefix string `yaml:"valuesPathPrefix,omitempty"`

	KubeContext string `yaml:"kubeContext,omitempty"`

	// InsecureSkipTLSVerify is true if the TLS verification should be skipped when fetching remote chart.
	InsecureSkipTLSVerify bool `yaml:"insecureSkipTLSVerify,omitempty"`

	// PlainHttp is true if the remote charte should be fetched using HTTP and not HTTPS
	PlainHttp bool `yaml:"plainHttp,omitempty"`

	// These values are used in templating
	VerifyTemplate    *string `yaml:"verifyTemplate,omitempty"`
	WaitTemplate      *string `yaml:"waitTemplate,omitempty"`
	InstalledTemplate *string `yaml:"installedTemplate,omitempty"`

	// These settings requires helm-x integration to work
	Dependencies          []Dependency `yaml:"dependencies,omitempty"`
	JSONPatches           []any        `yaml:"jsonPatches,omitempty"`
	StrategicMergePatches []any        `yaml:"strategicMergePatches,omitempty"`

	// Transformers is the list of Kustomize transformers
	//
	// Each item can be a path to a YAML or go template file, or an embedded transformer declaration as a YAML hash.
	// It's often used to add common labels and annotations to your resources.
	// See https://github.com/kubernetes-sigs/kustomize/blob/master/examples/configureBuiltinPlugin.md#configuring-the-builtin-plugins-instead for more information.
	Transformers []any    `yaml:"transformers,omitempty"`
	Adopt        []string `yaml:"adopt,omitempty"`

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

	// SkipRefresh disables running `helm dependency up`
	SkipRefresh *bool `yaml:"skipRefresh,omitempty"`

	// Propagate '--post-renderer' to helmv3 template and helm install
	PostRenderer *string `yaml:"postRenderer,omitempty"`

	// Propagate '--skip-schema-validation' to helmv3 template and helm install
	SkipSchemaValidation *bool `yaml:"skipSchemaValidation,omitempty"`

	// Propagate '--post-renderer-args' to helmv3 template and helm install
	PostRendererArgs []string `yaml:"postRendererArgs,omitempty"`

	// Cascade '--cascade' to helmv3 delete, available values: background, foreground, or orphan, default: background
	Cascade *string `yaml:"cascade,omitempty"`

	// SuppressOutputLineRegex is a list of regexes to suppress output lines
	SuppressOutputLineRegex []string `yaml:"suppressOutputLineRegex,omitempty"`

	// Inherit is used to inherit a release template from a release or another release template
	Inherit Inherits `yaml:"inherit,omitempty"`

	// SuppressDiff skip the helm diff output. Useful for charts which produces large not helpful diff.
	SuppressDiff *bool `yaml:"suppressDiff,omitempty"`

	// --wait flag for destroy/delete, if set to true, will wait until all resources are deleted before mark delete command as successful
	DeleteWait *bool `yaml:"deleteWait,omitempty"`
	// Timeout is the time in seconds to wait for helmfile delete command (default 300)
	DeleteTimeout *int `yaml:"deleteTimeout,omitempty"`
}

func (r *Inherits) UnmarshalYAML(unmarshal func(any) error) error {
	var v0151 []Inherit
	if err := unmarshal(&v0151); err != nil {
		var v0150 Inherit
		if err := unmarshal(&v0150); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "releases[].inherit of map(%+v) has been deprecated and will be removed in v0.152.0. Wrap it into an array: %v\n", v0150, err)
		*r = []Inherit{v0150}
		return nil
	}
	*r = v0151
	return nil
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
	Upgraded     []*ReleaseSpec
	Deleted      []*ReleaseSpec
	Failed       []*ReleaseSpec
	DeleteFailed []*ReleaseSpec
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

var DefaultFetchOutputDirTemplate = filepath.Join(
	"{{ .OutputDir }}{{ if .Release.Namespace }}",
	"{{ .Release.Namespace }}{{ end }}{{ if .Release.KubeContext }}",
	"{{ .Release.KubeContext }}{{ end }}",
	"{{ .Release.Name }}",
	"{{ .ChartName }}",
	"{{ or .Release.Version \"latest\" }}",
)

func (st *HelmState) reformat(spec *ReleaseSpec) []string {
	var needs []string
	releaseInstalledInfo := make(map[string]bool)
	for _, r := range st.OrginReleases {
		releaseInstalledInfo[r.Name] = r.Desired()
	}

	// Since the representation differs between needs and id,
	// correct it by prepending Namespace and KubeContext.
	for i := 0; i < len(spec.Needs); i++ {
		n := spec.Needs[i]

		var kubecontext, ns, name string

		components := strings.Split(n, "/")

		name = components[len(components)-1]
		if spec.Desired() && !releaseInstalledInfo[name] {
			st.logger.Warnf("WARNING: %s", fmt.Sprintf("release %s needs %s, but %s is not installed due to installed: false. Either mark %s as installed or remove %s from %s's needs", spec.Name, name, name, name, name, spec.Name))
		}

		if len(components) > 1 {
			ns = components[len(components)-2]
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
	return needs
}

func (st *HelmState) ApplyOverrides(spec *ReleaseSpec) {
	if st.OverrideKubeContext != "" {
		spec.KubeContext = st.OverrideKubeContext
	}
	if st.OverrideNamespace != "" {
		spec.Namespace = st.OverrideNamespace
	}

	spec.Needs = st.reformat(spec)
}

type RepoUpdater interface {
	IsHelm3() bool
	AddRepo(name, repository, cafile, certfile, keyfile, username, password string, managed string, passCredentials, skipTLSVerify bool) error
	UpdateRepo() error
	RegistryLogin(name, username, password, caFile, certFile, keyFile string, skipTLSVerify bool) error
}

func (st *HelmState) SyncRepos(helm RepoUpdater, shouldSkip map[string]bool) ([]string, error) {
	var updated []string

	for _, repo := range st.Repositories {
		if shouldSkip[repo.Name] {
			continue
		}
		username, password := gatherUsernamePassword(repo.Name, repo.Username, repo.Password)
		var err error
		if repo.OCI {
			err = helm.RegistryLogin(repo.URL, username, password, repo.CaFile, repo.CertFile, repo.KeyFile, repo.SkipTLSVerify)
		} else {
			err = helm.AddRepo(repo.Name, repo.URL, repo.CaFile, repo.CertFile, repo.KeyFile, username, password, repo.Managed, repo.PassCredentials, repo.SkipTLSVerify)
		}

		if err != nil {
			return nil, err
		}

		updated = append(updated, repo.Name)
	}

	return updated, nil
}

func gatherUsernamePassword(repoName string, username string, password string) (string, string) {
	var user, pass string

	replacedRepoName := strings.ToUpper(strings.Replace(repoName, "-", "_", -1))
	if username != "" {
		user = username
	} else if u := os.Getenv(fmt.Sprintf("%s_USERNAME", replacedRepoName)); u != "" {
		user = u
	}

	if password != "" {
		pass = password
	} else if p := os.Getenv(fmt.Sprintf("%s_PASSWORD", replacedRepoName)); p != "" {
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
				errs := []*ReleaseError{}
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
				flags, files, flagsErr := st.flagsForUpgrade(helm, release, workerIndex, opts)
				mut.Unlock()
				if flagsErr != nil {
					results <- syncPrepareResult{errors: []*ReleaseError{newReleaseFailedError(release, flagsErr)}, files: files}
					continue
				}

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

				flags = st.appendValuesControlModeFlag(flags, opts.ReuseValues, opts.ResetValues)

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
	if os.Getenv(envvar.UseHelmStatusToCheckReleaseExistence) != "" {
		st.logger.Debugf("Checking release existence using `helm status` for release %s", release.Name)

		flags := st.kubeConnectionFlags(&release)
		if release.Namespace != "" {
			flags = append(flags, "--namespace", release.Namespace)
		}
		err := helm.ReleaseStatus(context, release.Name, flags...)
		if err != nil && strings.Contains(err.Error(), "Error: release: not found") {
			return false, nil
		}
		return true, err
	}

	out, err := st.listReleases(context, helm, &release)
	if err != nil {
		return false, err
	}
	return out != "", nil
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
	Set                  []string
	SkipCleanup          bool
	SkipCRDs             bool
	Wait                 bool
	WaitRetries          int
	WaitForJobs          bool
	ReuseValues          bool
	ResetValues          bool
	PostRenderer         string
	SkipSchemaValidation bool
	PostRendererArgs     []string
	SyncArgs             string
	HideNotes            bool
	TakeOwnership        bool
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

	ns := r.Namespace

	if ns != "" {
		id += ns + "/"
	}

	if kc != "" {
		if ns == "" {
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

func (st *HelmState) appendDeleteWaitFlags(args []string, release *ReleaseSpec) []string {
	if release.DeleteWait != nil && *release.DeleteWait || release.DeleteWait == nil && st.HelmDefaults.DeleteWait {
		args = append(args, "--wait")
		timeout := st.HelmDefaults.DeleteTimeout
		if release.DeleteTimeout != nil {
			timeout = *release.DeleteTimeout
		}
		if timeout != 0 {
			duration := strconv.Itoa(timeout)
			duration += "s"
			args = append(args, "--timeout", duration)
		}
	}
	return args
}

// DeleteReleasesForSync deletes releases that are marked for deletion
func (st *HelmState) DeleteReleasesForSync(affectedReleases *AffectedReleases, helm helmexec.Interface, workerLimit int, cascade string) []error {
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
					if release.Namespace != "" {
						args = append(args, "--namespace", release.Namespace)
					}
					args = st.appendDeleteWaitFlags(args, release)
					args = st.appendConnectionFlags(args, release)
					deletionFlags := st.appendCascadeFlags(args, helm, release, cascade)

					m.Lock()
					start := time.Now()
					if _, err := st.triggerReleaseEvent("preuninstall", nil, release, "sync"); err != nil {
						affectedReleases.DeleteFailed = append(affectedReleases.Failed, release)
						relErr = newReleaseFailedError(release, err)
					} else if err := helm.DeleteRelease(context, release.Name, deletionFlags...); err != nil {
						affectedReleases.DeleteFailed = append(affectedReleases.Failed, release)
						relErr = newReleaseFailedError(release, err)
					} else if _, err := st.triggerReleaseEvent("postuninstall", nil, release, "sync"); err != nil {
						affectedReleases.DeleteFailed = append(affectedReleases.Failed, release)
						relErr = newReleaseFailedError(release, err)
					} else {
						affectedReleases.Deleted = append(affectedReleases.Deleted, release)
					}
					release.duration = time.Since(start)
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

				start := time.Now()
				if _, err := st.triggerPresyncEvent(release, "sync"); err != nil {
					relErr = newReleaseFailedError(release, err)
				} else if !release.Desired() {
					installed, err := st.isReleaseInstalled(context, helm, *release)
					if err != nil {
						relErr = newReleaseFailedError(release, err)
					} else if installed {
						var args []string
						deletionFlags := st.appendConnectionFlags(args, release)
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
				} else if err := helm.SyncRelease(context, release.Name, chart, release.Namespace, flags...); err != nil {
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
						st.logger.Debugf("getting deployed release version failed: %v", err)
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
				release.duration = time.Since(start)

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
	flags := st.kubeConnectionFlags(release)
	if release.Namespace != "" {
		flags = append(flags, "--namespace", release.Namespace)
	}
	flags = append(flags, "--uninstalling")
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
			chartMetadata, err := helm.ShowChart(release.Chart)
			if err != nil {
				return "failed to get version", errors.New("Failed to get the version for: " + chartName)
			}
			return chartMetadata.Version, nil
		}
	} else {
		return "failed to get version", err
	}
}

func releasesNeedCharts(releases []ReleaseSpec) []ReleaseSpec {
	var result []ReleaseSpec

	for _, r := range releases {
		if r.Desired() {
			result = append(result, r)
		}
	}

	return result
}

type ChartPrepareOptions struct {
	ForceDownload bool
	SkipRepos     bool
	SkipDeps      bool
	SkipRefresh   bool
	SkipResolve   bool
	SkipCleanup   bool
	// Validate is a helm-3-only option. When it is set to true, it configures chartify to pass --validate to helm-template run by it.
	// It's required when one of your chart relies on Capabilities.APIVersions in a template
	Validate               bool
	IncludeCRDs            *bool
	Wait                   bool
	WaitRetries            int
	WaitForJobs            bool
	OutputDir              string
	OutputDirTemplate      string
	IncludeTransitiveNeeds bool
	Concurrency            int
	KubeVersion            string
	Set                    []string
	Values                 []string
	TemplateArgs           string
	// Delete wait
	DeleteWait    bool
	DeleteTimeout int
}

type chartPrepareResult struct {
	releaseName            string
	releaseNamespace       string
	releaseContext         string
	chartName              string
	chartPath              string
	err                    error
	buildDeps              bool
	skipRefresh            bool
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

func (st *HelmState) Logger() *zap.SugaredLogger {
	return st.logger
}

type Chartifier interface {
	// Chartify creates a temporary Helm chart from a directory or a remote chart, and applies various transformations.
	// Returns the full path to the temporary directory containing the generated chart if succeeded.
	Chartify(release, dirOrChart string, opts ...chartify.ChartifyOption) (string, error)
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
// Otherwise, if a chart is not a helm chart, it will call "chartify" to turn it into a chart.
//
// If exists, it will also patch resources by json patches, strategic-merge patches, and injectors.
func (st *HelmState) PrepareCharts(helm helmexec.Interface, chartifier Chartifier, dir string, concurrency int, helmfileCommand string, opts ChartPrepareOptions) (map[PrepareChartKey]string, []error) {
	if !opts.SkipResolve {
		updated, err := st.ResolveDeps()
		if err != nil {
			return nil, []error{err}
		}
		*st = *updated
	}
	selected, err := st.GetSelectedReleases(opts.IncludeTransitiveNeeds)
	if err != nil {
		return nil, []error{err}
	}

	releases := releasesNeedCharts(selected)

	var prepareChartInfoMutex sync.Mutex

	prepareChartInfo := make(map[PrepareChartKey]string, len(releases))

	errs := []error{}

	jobQueue := make(chan *ReleaseSpec, len(releases))
	results := make(chan *chartPrepareResult, len(releases))

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
					ociChartPath, err := st.getOCIChart(release, dir, helm, opts.OutputDirTemplate)
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

				if chartification != nil && helmfileCommand != "pull" {
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

					if (helmfileCommand == "template" || helmfileCommand == "apply") && opts.TemplateArgs != "" {
						chartifyOpts.TemplateArgs = opts.TemplateArgs
					}

					chartifyOpts.KubeVersion = st.getKubeVersion(release, opts.KubeVersion)
					chartifyOpts.ApiVersions = st.getApiVersions(release)

					if opts.Values != nil {
						chartifyOpts.ValuesFiles = append(opts.Values, chartifyOpts.ValuesFiles...)
					}

					// https://github.com/helmfile/helmfile/pull/867
					// https://github.com/helmfile/helmfile/issues/895
					var flags []string
					for _, s := range opts.Set {
						flags = append(flags, "--set", s)
					}
					chartifyOpts.SetFlags = append(chartifyOpts.SetFlags, flags...)

					out, err := chartifier.Chartify(release.Name, chartPath, chartify.WithChartifyOpts(chartifyOpts))
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
					chartPath, err = generateChartPath(chartName, dir, release, opts.OutputDirTemplate)
					if err != nil {
						results <- &chartPrepareResult{err: err}
						return
					}

					// only fetch chart if it is not already fetched
					if _, err := os.Stat(chartPath); os.IsNotExist(err) {
						var fetchFlags []string
						fetchFlags = st.appendChartVersionFlags(fetchFlags, release)
						fetchFlags = append(fetchFlags, "--untar", "--untardir", chartPath)
						if err := helm.Fetch(chartName, fetchFlags...); err != nil {
							results <- &chartPrepareResult{err: err}
							return
						}
					} else {
						st.logger.Infof("\"%s\" has not been downloaded because the output directory \"%s\" already exists", chartName, chartPath)
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
					skipRefresh:            !isLocal || opts.SkipRefresh,
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
				func() {
					prepareChartInfoMutex.Lock()
					defer prepareChartInfoMutex.Unlock()
					prepareChartInfo[PrepareChartKey{
						Namespace:   downloadRes.releaseNamespace,
						KubeContext: downloadRes.releaseContext,
						Name:        downloadRes.releaseName,
					}] = downloadRes.chartPath
				}()

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

	return prepareChartInfo, nil
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
		buildDepsFlags := getBuildDepsFlags(r)
		if err := helm.BuildDeps(r.releaseName, r.chartPath, buildDepsFlags...); err != nil {
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
	NoHooks           bool
	SkipTests         bool
	PostRenderer      string
	PostRendererArgs  []string
	KubeVersion       string
	ShowOnly          []string
	// Propagate '--skip-schema-validation' to helmv3 template and helm install
	SkipSchemaValidation bool
	TemplateArgs         string
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

		flags, files, err := st.flagsForTemplate(helm, release, 0, opts)

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

		if opts.NoHooks {
			flags = append(flags, "--no-hooks")
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

		merged := map[string]any{}

		for _, f := range append(generatedFiles, additionalValues...) {
			src := map[string]any{}

			srcBytes, err := st.fs.ReadFile(f)
			if err != nil {
				return []error{fmt.Errorf("reading %s: %w", f, err)}
			}

			if err := yaml.Unmarshal(srcBytes, &src); err != nil {
				return []error{fmt.Errorf("unmarshalling yaml %s: %w", f, err)}
			}

			if err := mergo.Merge(&merged, &src, mergo.WithOverride); err != nil {
				return []error{fmt.Errorf("merging %s: %w", f, err)}
			}
		}

		var buf bytes.Buffer

		encoder := yaml.NewEncoder(&buf)
		if err := encoder.Encode(merged); err != nil {
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
	suppressDiff            bool
}

// commonDiffFlags returns common flags for helm diff, not in release-specific context
func (st *HelmState) commonDiffFlags(detailedExitCode bool, stripTrailingCR bool, includeTests bool, suppress []string, suppressSecrets bool, showSecrets bool, noHooks bool, opt *DiffOpts) []string {
	var flags []string

	if detailedExitCode {
		flags = append(flags, "--detailed-exitcode")
	}

	if stripTrailingCR {
		flags = append(flags, "--strip-trailing-cr")
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

	if opt.NoColor {
		flags = append(flags, "--no-color")
	} else if opt.Color {
		flags = append(flags, "--color")
	}

	if opt.Context > 0 {
		flags = append(flags, "--context", fmt.Sprintf("%d", opt.Context))
	}

	if opt.Output != "" {
		flags = append(flags, "--output", opt.Output)
	}

	flags = st.appendValuesControlModeFlag(flags, opt.ReuseValues, opt.ResetValues)

	if opt.Set != nil {
		for _, s := range opt.Set {
			flags = append(flags, "--set", s)
		}
	}
	flags = st.appendExtraDiffFlags(flags, opt)

	return flags
}

func (st *HelmState) prepareDiffReleases(helm helmexec.Interface, additionalValues []string, concurrency int, detailedExitCode bool, stripTrailingCR bool, includeTests bool, suppress []string, suppressSecrets bool, showSecrets bool, noHooks bool, opts ...DiffOpt) ([]diffPrepareResult, []error) {
	opt := &DiffOpts{}
	for _, o := range opts {
		o.Apply(opt)
	}

	mu := &sync.RWMutex{}
	installedReleases := map[string]bool{}

	isInstalled := func(r *ReleaseSpec) bool {
		id := ReleaseToID(r)

		mu.RLock()
		v, ok := installedReleases[id]
		mu.RUnlock()

		if ok {
			return v
		}

		v, err := st.isReleaseInstalled(st.createHelmContext(r, 0), helm, *r)
		if err != nil {
			st.logger.Warnf("confirming if the release is already installed or not: %v", err)
		} else {
			mu.Lock()
			installedReleases[id] = v
			mu.Unlock()
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
	commonDiffFlags := st.commonDiffFlags(detailedExitCode, stripTrailingCR, includeTests, suppress, suppressSecrets, showSecrets, noHooks, opt)

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

				suppressDiff := false
				if release.SuppressDiff != nil && *release.SuppressDiff {
					suppressDiff = true
				}

				if opt.SkipDiffOnInstall && !isInstalled(release) {
					results <- diffPrepareResult{release: release, upgradeDueToSkippedDiff: true, suppressDiff: suppressDiff}
					continue
				}

				disableValidation := release.DisableValidationOnInstall != nil && *release.DisableValidationOnInstall && !isInstalled(release)

				// TODO We need a long-term fix for this :)
				// See https://github.com/roboll/helmfile/issues/737
				mut.Lock()
				// release level diff flags in here
				flags, files, err := st.flagsForDiff(helm, release, disableValidation, workerIndex, opt)
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

				flags = append(flags, commonDiffFlags...)

				if len(errs) > 0 {
					rsErrs := make([]*ReleaseError, len(errs))
					for i, e := range errs {
						rsErrs[i] = newReleaseFailedError(release, e)
					}
					results <- diffPrepareResult{errors: rsErrs, files: files, suppressDiff: suppressDiff}
				} else {
					results <- diffPrepareResult{release: release, flags: flags, errors: []*ReleaseError{}, files: files, suppressDiff: suppressDiff}
				}
			}
		},
		func() {
			for i := 0; i < numReleases; i++ {
				res := <-results
				if len(res.errors) > 0 {
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
	historyMax := 10
	if st.HelmDefaults.HistoryMax != nil {
		historyMax = *st.HelmDefaults.HistoryMax
	}
	if spec.HistoryMax != nil {
		historyMax = *spec.HistoryMax
	}

	return helmexec.HelmContext{
		WorkerIndex: workerIndex,
		HistoryMax:  historyMax,
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
	NoColor                 bool
	Set                     []string
	SkipCleanup             bool
	SkipDiffOnInstall       bool
	DiffArgs                string
	ReuseValues             bool
	ResetValues             bool
	PostRenderer            string
	PostRendererArgs        []string
	SuppressOutputLineRegex []string
	SkipSchemaValidation    bool
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
func (st *HelmState) DiffReleases(helm helmexec.Interface, additionalValues []string, workerLimit int, detailedExitCode bool, stripTrailingCR bool, includeTests bool, suppress []string, suppressSecrets, showSecrets, noHooks bool, suppressDiff, triggerCleanupEvents bool, opt ...DiffOpt) ([]ReleaseSpec, []error) {
	opts := &DiffOpts{}
	for _, o := range opt {
		o.Apply(opts)
	}

	preps, prepErrs := st.prepareDiffReleases(helm, additionalValues, workerLimit, detailedExitCode, stripTrailingCR, includeTests, suppress, suppressSecrets, showSecrets, noHooks, opts)

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

				releaseSuppressDiff := suppressDiff
				if prep.suppressDiff {
					releaseSuppressDiff = true
				}

				if prep.upgradeDueToSkippedDiff {
					results <- diffResult{release, &ReleaseError{ReleaseSpec: release, err: nil, Code: HelmDiffExitCodeChanged}, buf}
				} else if err := helm.DiffRelease(st.createHelmContextWithWriter(release, buf), release.Name, normalizeChart(st.basePath, release.ChartPathOrName()), release.Namespace, releaseSuppressDiff, flags...); err != nil {
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
		if release.Namespace != "" {
			flags = append(flags, "--namespace", release.Namespace)
		}
		flags = st.appendConnectionFlags(flags, &release)

		return helm.ReleaseStatus(st.createHelmContext(&release, workerIndex), release.Name, flags...)
	})
}

// DeleteReleases wrapper for executing helm delete on the releases
func (st *HelmState) DeleteReleases(affectedReleases *AffectedReleases, helm helmexec.Interface, concurrency int, purge bool, cascade string) []error {
	return st.scatterGatherReleases(helm, concurrency, func(release ReleaseSpec, workerIndex int) error {
		st.ApplyOverrides(&release)

		flags := make([]string, 0)
		flags = st.appendConnectionFlags(flags, &release)
		flags = st.appendCascadeFlags(flags, helm, &release, cascade)
		flags = st.appendDeleteWaitFlags(flags, &release)
		if release.Namespace != "" {
			flags = append(flags, "--namespace", release.Namespace)
		}
		context := st.createHelmContext(&release, workerIndex)

		start := time.Now()
		if _, err := st.triggerReleaseEvent("preuninstall", nil, &release, "delete"); err != nil {
			release.duration = time.Since(start)

			affectedReleases.DeleteFailed = append(affectedReleases.Failed, &release)

			return err
		}

		if err := helm.DeleteRelease(context, release.Name, flags...); err != nil {
			release.duration = time.Since(start)

			affectedReleases.DeleteFailed = append(affectedReleases.Failed, &release)
			return err
		}

		if _, err := st.triggerReleaseEvent("postuninstall", nil, &release, "delete"); err != nil {
			release.duration = time.Since(start)

			affectedReleases.DeleteFailed = append(affectedReleases.Failed, &release)
			return err
		}
		release.duration = time.Since(start)

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
		if release.Namespace != "" {
			flags = append(flags, "--namespace", release.Namespace)
		}
		if opts.Logs {
			flags = append(flags, "--logs")
		}

		if timeout == EmptyTimeout {
			flags = append(flags, st.timeoutFlags(&release)...)
		} else {
			duration := strconv.Itoa(timeout)
			duration += "s"
			flags = append(flags, "--timeout", duration)
		}

		flags = st.appendConnectionFlags(flags, &release)
		flags = st.appendChartDownloadFlags(flags, &release)

		return helm.TestRelease(st.createHelmContext(&release, workerIndex), release.Name, flags...)
	})
}

// Clean will remove any generated secrets
func (st *HelmState) Clean() []error {
	return nil
}

func (st *HelmState) GetReleasesWithOverrides() ([]ReleaseSpec, error) {
	var rs []ReleaseSpec
	for _, r := range st.Releases {
		spec := r
		st.ApplyOverrides(&spec)
		rs = append(rs, spec)
	}
	return rs, nil
}

func (st *HelmState) SelectReleases(includeTransitiveNeeds bool) ([]Release, error) {
	values := st.Values()
	rs, err := markExcludedReleases(st.Releases, st.Selectors, st.CommonLabels, values, includeTransitiveNeeds)
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func markExcludedReleases(releases []ReleaseSpec, selectors []string, commonLabels map[string]string, values map[string]any, includeTransitiveNeeds bool) ([]Release, error) {
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
		orginReleaseLabel := maps.Clone(r.Labels)
		if r.Labels == nil {
			r.Labels = map[string]string{}
		} else {
			// Make a copy of the labels to avoid mutating the original
			r.Labels = maps.Clone(r.Labels)
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
		// reset the labels to the original
		r.Labels = orginReleaseLabel
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

// ConditionEnabled checks if a release condition is enabled based on the provided values.
// It takes a ReleaseSpec and a map of values as input.
// If the condition is not specified, it returns true.
// If the condition is specified but not in the form 'foo.enabled', it returns an error.
// If the condition is specified and the corresponding value is found in the values map,
// it checks if the 'enabled' field is set to true. If so, it returns true.
// Otherwise, it returns false.
// If the condition is specified but the corresponding value is not found in the values map,
// it returns an error.
func ConditionEnabled(r ReleaseSpec, values map[string]any) (bool, error) {
	if len(r.Condition) == 0 {
		return true, nil
	}
	iValues := values
	keys := strings.Split(r.Condition, ".")
	if keys[len(keys)-1] != "enabled" {
		return false, fmt.Errorf("Condition value must be in the form 'foo.enabled' where 'foo' can be modified as necessary")
	}

	currentKey := ""
	for _, key := range keys[:len(keys)-1] {
		currentKey = fmt.Sprintf("%s.%s", currentKey, key)
		value, ok := iValues[key]
		if !ok {
			return false, fmt.Errorf("environment values field '%s' not found", currentKey)
		}

		iValues, ok = value.(map[string]interface{})
		if !ok {
			return false, fmt.Errorf("environment values field '%s' is not a map", currentKey)
		}
	}

	enabled, ok := iValues["enabled"]

	if !ok {
		return false, nil
	}

	e, ok := enabled.(bool)

	if !ok {
		return false, nil
	}

	return e, nil
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

func (st *HelmState) GetSelectedReleases(includeTransitiveNeeds bool) ([]ReleaseSpec, error) {
	filteredReleases, err := st.SelectReleases(includeTransitiveNeeds)
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
	releases, err := st.GetSelectedReleases(includeTransitiveNeeds)
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
	data := map[string]any{
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
	data := map[string]any{
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
		selected, err = st.GetSelectedReleases(includeTransitiveNeeds)
		if err != nil {
			return []error{err}
		}
	} else {
		selected = st.Releases
	}

	releases := releasesNeedCharts(selected)

	var errs []error

	for _, release := range releases {
		if !st.fs.DirectoryExistsAt(release.ChartPathOrName()) {
			st.logger.Debugf("skipped updating dependencies for remote chart %s", release.Chart)
		} else if !st.fs.FileExistsAt(filepath.Join(release.ChartPathOrName(), "Chart.yaml")) {
			st.logger.Debugf("skipped updating dependencies for %s as it does not have a Chart.yaml", release.Chart)
		} else {
			if err := helm.UpdateDeps(release.ChartPathOrName()); err != nil {
				errs = append(errs, err)
			}
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

// appendConnectionFlags append all the helm command-line flags related to K8s API including the kubecontext
func (st *HelmState) appendConnectionFlags(flags []string, release *ReleaseSpec) []string {
	kubeFlagAdds := st.kubeConnectionFlags(release)
	flags = append(flags, kubeFlagAdds...)
	return flags
}

// appendExtraDiffFlags appends extra diff flags to the given flags slice based on the provided options.
// If opt is not nil and opt.DiffArgs is not empty, it collects the arguments from opt.DiffArgs and appends them to flags.
// If st.HelmDefaults.DiffArgs is not nil, it joins the arguments with a space and appends them to flags.
// The updated flags slice is returned.
func (st *HelmState) appendExtraDiffFlags(flags []string, opt *DiffOpts) []string {
	switch {
	case opt != nil && opt.DiffArgs != "":
		flags = append(flags, argparser.CollectArgs(opt.DiffArgs)...)
	case st.HelmDefaults.DiffArgs != nil:
		flags = append(flags, argparser.CollectArgs(strings.Join(st.HelmDefaults.DiffArgs, " "))...)
	}
	return flags
}

// appendExtraSyncFlags appends extra diff flags to the given flags slice based on the provided options.
// If opt is not nil and opt.SyncArgs is not empty, it collects the arguments from opt.SyncArgs and appends them to flags.
// If st.HelmDefaults.SyncArgs is not nil, it joins the arguments with a space and appends them to flags.
// The updated flags slice is returned.
func (st *HelmState) appendExtraSyncFlags(flags []string, opt *SyncOpts) []string {
	switch {
	case opt != nil && opt.SyncArgs != "":
		flags = append(flags, argparser.CollectArgs(opt.SyncArgs)...)
	case st.HelmDefaults.SyncArgs != nil:
		flags = append(flags, argparser.CollectArgs(strings.Join(st.HelmDefaults.SyncArgs, " "))...)
	}
	return flags
}

// appendVerifyFlags append the --verify flags related to verify
func (st *HelmState) appendVerifyFlags(flags []string, release *ReleaseSpec) []string {
	repo, _ := st.GetRepositoryAndNameFromChartName(release.Chart)
	switch {
	case release.Verify != nil:
		// If the release has a verify flag, use it
		v := *release.Verify
		if v {
			flags = append(flags, "--verify")
		}
		return flags
	case repo != nil && repo.Verify:
		flags = append(flags, "--verify")
	case st.HelmDefaults.Verify:
		flags = append(flags, "--verify")
	}
	return flags
}

// appendKeyringFlags append all the helm command-line flags related to keyring
func (st *HelmState) appendKeyringFlags(flags []string, release *ReleaseSpec) []string {
	repo, _ := st.GetRepositoryAndNameFromChartName(release.Chart)
	switch {
	case release.Keyring != "":
		flags = append(flags, "--keyring", release.Keyring)
	case repo != nil && repo.Keyring != "":
		flags = append(flags, "--keyring", repo.Keyring)
	case st.HelmDefaults.Keyring != "":
		flags = append(flags, "--keyring", st.HelmDefaults.Keyring)
	}

	return flags
}

func (st *HelmState) kubeConnectionFlags(release *ReleaseSpec) []string {
	flags := []string{}
	if release.KubeContext != "" {
		flags = append(flags, "--kube-context", release.KubeContext)
	} else if st.Environments[st.Env.Name].KubeContext != "" {
		flags = append(flags, "--kube-context", st.Environments[st.Env.Name].KubeContext)
	} else if st.HelmDefaults.KubeContext != "" {
		flags = append(flags, "--kube-context", st.HelmDefaults.KubeContext)
	}
	return flags
}

func (st *HelmState) appendChartDownloadFlags(flags []string, release *ReleaseSpec) []string {
	repo, _ := st.GetRepositoryAndNameFromChartName(release.Chart)
	if st.needsPlainHttp(release, repo) {
		flags = append(flags, "--plain-http")
		// --insecure-skip-tls-verify nullifies --plain-http in helm, omit it if PlainHttp is specified
		return flags
	}

	if st.needsInsecureSkipTLSVerify(release, repo) {
		flags = append(flags, "--insecure-skip-tls-verify")
	}

	return flags
}

func (st *HelmState) needsPlainHttp(release *ReleaseSpec, repo *RepositorySpec) bool {
	var repoPlainHttp, relPlainHttp bool
	if repo != nil {
		repoPlainHttp = repo.PlainHttp
	}

	if release != nil {
		relPlainHttp = release.PlainHttp
	}

	return relPlainHttp || st.HelmDefaults.PlainHttp || repoPlainHttp
}

func (st *HelmState) needsInsecureSkipTLSVerify(release *ReleaseSpec, repo *RepositorySpec) bool {
	var repoSkipTLSVerify, relSkipTLSVerify bool
	if repo != nil {
		repoSkipTLSVerify = repo.SkipTLSVerify
	}

	if release != nil {
		relSkipTLSVerify = release.InsecureSkipTLSVerify
	}

	return relSkipTLSVerify || st.HelmDefaults.InsecureSkipTLSVerify || repoSkipTLSVerify
}

func (st *HelmState) timeoutFlags(release *ReleaseSpec) []string {
	var flags []string

	timeout := st.HelmDefaults.Timeout
	if release.Timeout != nil {
		timeout = *release.Timeout
	}
	if timeout != 0 {
		duration := strconv.Itoa(timeout)
		duration += "s"
		flags = append(flags, "--timeout", duration)
	}

	return flags
}

func (st *HelmState) flagsForUpgrade(helm helmexec.Interface, release *ReleaseSpec, workerIndex int, opt *SyncOpts) ([]string, []string, error) {
	var flags []string
	flags = st.appendChartVersionFlags(flags, release)
	if release.EnableDNS != nil && *release.EnableDNS || release.EnableDNS == nil && st.HelmDefaults.EnableDNS {
		flags = append(flags, "--enable-dns")
	}

	flags = st.appendWaitFlags(flags, helm, release, opt)
	flags = st.appendWaitForJobsFlags(flags, release, opt)

	// non-OCI chart should be verified here
	if !st.IsOCIChart(release.Chart) {
		flags = st.appendVerifyFlags(flags, release)
		flags = st.appendKeyringFlags(flags, release)
	}

	flags = append(flags, st.timeoutFlags(release)...)

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

	flags = st.appendConnectionFlags(flags, release)
	flags = st.appendChartDownloadFlags(flags, release)

	flags = st.appendHelmXFlags(flags, release)

	postRenderer := ""
	if opt != nil {
		postRenderer = opt.PostRenderer
	}
	flags = st.appendPostRenderFlags(flags, release, postRenderer)

	var postRendererArgs []string
	if opt != nil {
		postRendererArgs = opt.PostRendererArgs
	}
	flags = st.appendPostRenderArgsFlags(flags, release, postRendererArgs)

	skipSchemaValidation := false
	if opt != nil {
		skipSchemaValidation = opt.SkipSchemaValidation
	}

	flags = st.appendSkipSchemaValidationFlags(flags, release, skipSchemaValidation)

	// append hide-notes flag
	flags = st.appendHideNotesFlags(flags, helm, opt)

	// append take-ownership flag
	flags = st.appendTakeOwnershipFlags(flags, helm, opt)

	flags = st.appendExtraSyncFlags(flags, opt)

	common, clean, err := st.namespaceAndValuesFlags(helm, release, workerIndex)
	if err != nil {
		return nil, clean, err
	}
	return append(flags, common...), clean, nil
}

func (st *HelmState) flagsForTemplate(helm helmexec.Interface, release *ReleaseSpec, workerIndex int, opt *TemplateOpts) ([]string, []string, error) {
	var flags []string
	flags = st.appendChartVersionFlags(flags, release)
	flags = st.appendHelmXFlags(flags, release)

	var postRendererArgs []string
	var showOnly []string
	postRenderer := ""
	kubeVersion := ""
	skipSchemaValidation := false
	if opt != nil {
		postRenderer = opt.PostRenderer
		postRendererArgs = opt.PostRendererArgs
		kubeVersion = opt.KubeVersion
		showOnly = opt.ShowOnly
		skipSchemaValidation = opt.SkipSchemaValidation
	}
	flags = st.appendPostRenderFlags(flags, release, postRenderer)
	flags = st.appendPostRenderArgsFlags(flags, release, postRendererArgs)
	flags = st.appendApiVersionsFlags(flags, release, kubeVersion)
	flags = st.appendChartDownloadFlags(flags, release)
	flags = st.appendShowOnlyFlags(flags, showOnly)
	flags = st.appendSkipSchemaValidationFlags(flags, release, skipSchemaValidation)

	common, files, err := st.namespaceAndValuesFlags(helm, release, workerIndex)
	if err != nil {
		return nil, files, err
	}
	return append(flags, common...), files, nil
}

func (st *HelmState) flagsForDiff(helm helmexec.Interface, release *ReleaseSpec, disableValidation bool, workerIndex int, opt *DiffOpts) ([]string, []string, error) {
	settings := cli.New()
	var flags []string
	flags = st.appendChartVersionFlags(flags, release)

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

	// TODO:
	// `helm diff` has `--kube-version` flag from v3.5.0, but only respected when `helm diff upgrade --disable-validation`.
	// `helm template --validate` and `helm upgrade --dry-run` ignore `--kube-version` flag.
	// For the moment, not specifying kubeVersion.
	flags = st.appendApiVersionsFlags(flags, release, "")
	flags = st.appendConnectionFlags(flags, release)
	flags = st.appendChartDownloadFlags(flags, release)

	// `helm diff` does not support the `--plain-http` flag, this needs to be removed
	repo, _ := st.GetRepositoryAndNameFromChartName(release.Chart)
	if st.needsPlainHttp(release, repo) {
		var cleanFlags []string
		for _, flag := range flags {
			if flag != "--plain-http" {
				cleanFlags = append(cleanFlags, flag)
			}
		}
		flags = cleanFlags
	}

	for _, flag := range flags {
		if flag == "--insecure-skip-tls-verify" {
			diffVersion, err := helmexec.GetPluginVersion("diff", settings.PluginsDirectory)
			if err != nil {
				return nil, nil, err
			}
			dv, _ := semver.NewVersion("v3.8.1")

			if diffVersion.LessThan(dv) {
				return nil, nil, fmt.Errorf("insecureSkipTLSVerify is not supported by helm-diff plugin version %s, please use at least v3.8.1", diffVersion)
			}

			break
		}
	}

	flags = st.appendHelmXFlags(flags, release)

	postRenderer := ""
	if opt != nil {
		postRenderer = opt.PostRenderer
	}
	flags = st.appendPostRenderFlags(flags, release, postRenderer)

	postRendererArgs := []string{}
	if opt != nil {
		postRendererArgs = opt.PostRendererArgs
	}
	flags = st.appendPostRenderArgsFlags(flags, release, postRendererArgs)

	skipSchemaValidation := false
	if opt != nil {
		skipSchemaValidation = opt.SkipSchemaValidation
	}
	flags = st.appendSkipSchemaValidationFlags(flags, release, skipSchemaValidation)

	suppressOutputLineRegex := []string{}
	if opt != nil {
		suppressOutputLineRegex = opt.SuppressOutputLineRegex
	}
	if len(suppressOutputLineRegex) > 0 || len(st.HelmDefaults.SuppressOutputLineRegex) > 0 || len(release.SuppressOutputLineRegex) > 0 {
		diffVersion, err := helmexec.GetPluginVersion("diff", settings.PluginsDirectory)
		if err != nil {
			return nil, nil, err
		}
		dv, _ := semver.NewVersion("v3.9.0")

		if diffVersion.LessThan(dv) {
			return nil, nil, fmt.Errorf("suppressOutputLineRegex is not supported by helm-diff plugin version %s, please use at least v3.9.0", diffVersion)
		}
		flags = st.appendSuppressOutputLineRegexFlags(flags, release, suppressOutputLineRegex)
	}

	common, files, err := st.namespaceAndValuesFlags(helm, release, workerIndex)
	if err != nil {
		return nil, files, err
	}

	return append(flags, common...), files, nil
}

func (st *HelmState) appendChartVersionFlags(flags []string, release *ReleaseSpec) []string {
	if release.Version != "" {
		flags = append(flags, "--version", release.Version)
	}

	if st.isDevelopment(release) {
		flags = append(flags, "--devel")
	}

	return flags
}

func (st *HelmState) chartOCIFlags(r *ReleaseSpec) []string {
	flags := []string{}
	repo, _ := st.GetRepositoryAndNameFromChartName(r.Chart)
	if repo != nil {
		if repo.PlainHttp {
			flags = append(flags, "--plain-http")
		} else {
			// TLS options will nullify --plain-http in helm if passed, omit it them PlainHttp is specified
			if repo.SkipTLSVerify {
				flags = append(flags, "--insecure-skip-tls-verify")
			}
			if repo.CaFile != "" {
				flags = append(flags, "--ca-file", repo.CaFile)
			}
			if repo.CertFile != "" && repo.KeyFile != "" {
				flags = append(flags, "--cert-file", repo.CertFile, "--key-file", repo.KeyFile)
			}
		}
		if repo.Verify {
			flags = append(flags, "--verify")
		}
		if repo.Keyring != "" {
			flags = append(flags, "--keyring", repo.Keyring)
		}
		if repo.RegistryConfig != "" {
			flags = append(flags, "--registry-config", repo.RegistryConfig)
		}
	}

	return flags
}

func (st *HelmState) appendValuesControlModeFlag(flags []string, reuseValues bool, resetValues bool) []string {
	if !resetValues && (st.HelmDefaults.ReuseValues || reuseValues) {
		flags = append(flags, "--reuse-values")
	} else {
		flags = append(flags, "--reset-values")
	}

	return flags
}

func (st *HelmState) getApiVersions(r *ReleaseSpec) []string {
	if len(r.ApiVersions) != 0 {
		return r.ApiVersions
	}
	return st.ApiVersions
}

func (st *HelmState) getKubeVersion(r *ReleaseSpec, kubeVersion string) string {
	switch {
	case kubeVersion != "":
		return kubeVersion
	case r.KubeVersion != "":
		return r.KubeVersion
	case st.KubeVersion != "":
		return st.KubeVersion
	}
	return ""
}

func (st *HelmState) appendApiVersionsFlags(flags []string, r *ReleaseSpec, kubeVersion string) []string {
	for _, a := range st.getApiVersions(r) {
		flags = append(flags, "--api-versions", a)
	}

	if version := st.getKubeVersion(r, kubeVersion); version != "" {
		flags = append(flags, "--kube-version", version)
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

	return st.appendHelmXFlags(flags, release), files, nil
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
		var rawYaml map[string]any

		if err := yaml.Unmarshal(rawBytes, &rawYaml); err != nil {
			return nil, err
		}

		parsedYaml, err := st.valsRuntime.Eval(rawYaml)
		if err != nil {
			return nil, err
		}

		return yaml.Marshal(parsedYaml)
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
	dirsToClean := map[string]int{}
	for _, f := range files {
		dirsToClean[filepath.Dir(f)] = 1
		if err := st.fs.DeleteFile(f); err != nil {
			st.logger.Warnf("Removing %s: %v", f, err)
		} else {
			st.logger.Debugf("Removed %s", f)
		}
	}
	for d := range dirsToClean {
		// check if the directory is empty
		des, err := st.fs.ReadDir(d)
		if err != nil {
			st.logger.Warnf("Reading dir %s: %v", d, err)
			continue
		}

		if len(des) > 0 {
			st.logger.Debugf("Not removing %s because it's not empty", d)
			continue
		}

		if err := st.fs.DeleteFile(d); err != nil {
			st.logger.Warnf("Removing %s: %v", d, err)
		} else {
			st.logger.Debugf("Removed %s", d)
		}
	}
}

func (c MissingFileHandlerConfig) resolveFileOptions() []resolveFileOption {
	return []resolveFileOption{
		ignoreMissingGitBranch(c.IgnoreMissingGitBranch),
	}
}

func (st *HelmState) generateTemporaryReleaseValuesFiles(release *ReleaseSpec, values []any, missingFileHandler *string) ([]string, error) {
	generatedFiles := []string{}

	for _, value := range values {
		switch typedValue := value.(type) {
		case string:
			paths, skip, err := st.storage().resolveFile(missingFileHandler, "values", typedValue, st.MissingFileHandlerConfig.resolveFileOptions()...)
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
		case map[any]any, map[string]any:
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
	values := []any{}
	for _, v := range release.Values {
		switch typedValue := v.(type) {
		case string:
			path := st.storage().normalizePath(release.ValuesPathPrefix + typedValue)
			values = append(values, path)
		default:
			values = append(values, v)
		}
	}

	valuesMapSecretsRendered, err := st.valsRuntime.Eval(map[string]any{"values": values})
	if err != nil {
		return nil, err
	}

	valuesSecretsRendered, ok := valuesMapSecretsRendered["values"].([]any)
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
	var generatedDecryptedFiles []any

	for _, v := range release.Secrets {
		var (
			paths []string
			skip  bool
			err   error
		)

		switch value := v.(type) {
		case string:
			paths, skip, err = st.storage().resolveFile(release.MissingFileHandler, "secrets", release.ValuesPathPrefix+value, st.MissingFileHandlerConfig.resolveFileOptions()...)
			if err != nil {
				return nil, err
			}
		default:
			bs, err := yaml.Marshal(value)
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

		valfile, err := helm.DecryptSecret(st.createHelmContext(release, workerIndex), path)
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

	if len(release.SetStringValues) > 0 {
		setStringFlags, err := st.setStringFlags(release.SetStringValues)
		if err != nil {
			return nil, files, fmt.Errorf("Failed to render set string value entry in %s for release %s: %v", st.FilePath, release.Name, err)
		}

		flags = append(flags, setStringFlags...)
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
			flags = append(flags, "--set-file", fmt.Sprintf("%s=%s", escape(set.Name), st.storage().normalizeSetFilePath(set.File, runtime.GOOS)))
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

// setStringFlags is to generate the set-string flags for helm
func (st *HelmState) setStringFlags(setValues []SetValue) ([]string, error) {
	var flags []string

	for _, set := range setValues {
		if set.Value != "" {
			renderedValue, err := renderValsSecrets(st.valsRuntime, set.Value)
			if err != nil {
				return nil, err
			}
			flags = append(flags, "--set-string", fmt.Sprintf("%s=%s", escape(set.Name), escape(renderedValue[0])))
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
			flags = append(flags, "--set-string", fmt.Sprintf("%s={%s}", escape(set.Name), v))
		}
	}

	return flags, nil
}

// renderValsSecrets helper function which renders 'ref+.*' secrets
func renderValsSecrets(e vals.Evaluator, input ...string) ([]string, error) {
	output := make([]string, len(input))
	if len(input) > 0 {
		mapRendered, err := e.Eval(map[string]any{"values": input})
		if err != nil {
			return nil, err
		}

		rendered, ok := mapRendered["values"].([]any)
		if !ok {
			return nil, fmt.Errorf("type %T isn't supported", mapRendered["values"])
		}

		for i := 0; i < len(rendered); i++ {
			output[i] = fmt.Sprintf("%v", rendered[i])
		}
	}
	return output, nil
}

func hideChartCredentials(chartCredentials string) (string, error) {
	u, err := url.Parse(chartCredentials)
	if err != nil {
		return "", err
	}
	if u.User != nil {
		u.User = url.UserPassword("---", "---")
	}
	modifiedURL := u.String()
	return modifiedURL, nil
}

// DisplayAffectedReleases logs the upgraded, deleted and in error releases
func (ar *AffectedReleases) DisplayAffectedReleases(logger *zap.SugaredLogger) {
	if len(ar.Upgraded) > 0 {
		logger.Info("\nUPDATED RELEASES:")
		tbl, _ := prettytable.NewTable(prettytable.Column{Header: "NAME"},
			prettytable.Column{Header: "NAMESPACE", MinWidth: 6},
			prettytable.Column{Header: "CHART", MinWidth: 6},
			prettytable.Column{Header: "VERSION", MinWidth: 6},
			prettytable.Column{Header: "DURATION", AlignRight: true},
		)
		tbl.Separator = "   "
		for _, release := range ar.Upgraded {
			modifiedChart, modErr := hideChartCredentials(release.Chart)
			if modErr != nil {
				logger.Warn("Could not modify chart credentials, %v", modErr)
				continue
			}
			err := tbl.AddRow(release.Name, release.Namespace, modifiedChart, release.installedVersion, release.duration.Round(time.Second))
			if err != nil {
				logger.Warn("Could not add row, %v", err)
			}
		}
		logger.Info(tbl.String())
	}
	if len(ar.Deleted) > 0 {
		logger.Info("\nDELETED RELEASES:")
		tbl, _ := prettytable.NewTable(prettytable.Column{Header: "NAME"},
			prettytable.Column{Header: "NAMESPACE", MinWidth: 6},
			prettytable.Column{Header: "DURATION", AlignRight: true},
		)
		tbl.Separator = "   "
		for _, release := range ar.Deleted {
			err := tbl.AddRow(release.Name, release.Namespace, release.duration.Round(time.Second))
			if err != nil {
				logger.Warn("Could not add row, %v", err)
			}
		}
		logger.Info(tbl.String())
	}
	if len(ar.Failed) > 0 {
		logger.Info("\nFAILED RELEASES:")
		tbl, _ := prettytable.NewTable(prettytable.Column{Header: "NAME"},
			prettytable.Column{Header: "NAMESPACE", MinWidth: 6},
			prettytable.Column{Header: "CHART", MinWidth: 6},
			prettytable.Column{Header: "VERSION", MinWidth: 6},
			prettytable.Column{Header: "DURATION", AlignRight: true},
		)
		tbl.Separator = "   "
		for _, release := range ar.Failed {
			err := tbl.AddRow(release.Name, release.Namespace, release.Chart, release.installedVersion, release.duration.Round(time.Second))
			if err != nil {
				logger.Warn("Could not add row, %v", err)
			}
		}
		logger.Info(tbl.String())
	}
	if len(ar.DeleteFailed) > 0 {
		logger.Info("\nFAILED TO DELETE RELEASES:")
		tbl, _ := prettytable.NewTable(prettytable.Column{Header: "NAME"},
			prettytable.Column{Header: "NAMESPACE", MinWidth: 6},
			prettytable.Column{Header: "DURATION", AlignRight: true},
		)
		tbl.Separator = "   "
		for _, release := range ar.DeleteFailed {
			err := tbl.AddRow(release.Name, release.Namespace, release.duration.Round(time.Second))
			if err != nil {
				logger.Warn("Could not add row, %v", err)
			}
		}
		logger.Info(tbl.String())
	}
}

func escape(value string) string {
	intermediate := strings.ReplaceAll(value, "{", "\\{")
	intermediate = strings.ReplaceAll(intermediate, "}", "\\}")
	return strings.ReplaceAll(intermediate, ",", "\\,")
}

// MarshalYAML will ensure we correctly marshal SubHelmfileSpec structure correctly so it can be unmarshalled at some
// future time
func (p SubHelmfileSpec) MarshalYAML() (any, error) {
	type SubHelmfileSpecTmp struct {
		Path               string   `yaml:"path,omitempty"`
		Selectors          []string `yaml:"selectors,omitempty"`
		SelectorsInherited bool     `yaml:"selectorsInherited,omitempty"`
		OverrideValues     []any    `yaml:"values,omitempty"`
	}
	return &SubHelmfileSpecTmp{
		Path:               p.Path,
		Selectors:          p.Selectors,
		SelectorsInherited: p.SelectorsInherited,
		OverrideValues:     p.Environment.OverrideValues,
	}, nil
}

// UnmarshalYAML will unmarshal the helmfile yaml section and fill the SubHelmfileSpec structure
// this is required go-yto keep allowing string scalar for defining helmfile
func (hf *SubHelmfileSpec) UnmarshalYAML(unmarshal func(any) error) error {
	var tmp any
	if err := unmarshal(&tmp); err != nil {
		return err
	}

	switch i := tmp.(type) {
	case string: // single path definition without sub items, legacy sub helmfile definition
		hf.Path = i
	case map[any]any, map[string]any: // helmfile path with sub section
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

// generateChartPath generates the path of the output directory of the `helmfile fetch` command.
// It uses a go template with data from the chart name, output directory and release spec.
// If no template was provided (via the `--output-dir-template` flag) it uses the DefaultFetchOutputDirTemplate.
func generateChartPath(chartName string, outputDir string, release *ReleaseSpec, outputDirTemplate string) (string, error) {
	if outputDirTemplate == "" {
		outputDirTemplate = DefaultFetchOutputDirTemplate
	}

	t, err := template.New("output-dir-template").Parse(outputDirTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing output-dir-template template %q: %w", outputDirTemplate, err)
	}

	buf := &bytes.Buffer{}
	data := struct {
		ChartName string
		OutputDir string
		Release   ReleaseSpec
	}{
		ChartName: chartName,
		OutputDir: outputDir,
		Release:   *release,
	}
	if err := t.Execute(buf, data); err != nil {
		return "", fmt.Errorf("executing output-dir-template template: %w", err)
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
		return "", fmt.Errorf("parsing output-file template %q: %w", outputFileTemplate, err)
	}

	buf := &bytes.Buffer{}

	type state struct {
		BaseName    string
		Path        string
		AbsPath     string
		AbsPathSHA1 string
	}

	data := struct {
		State       state
		Release     *ReleaseSpec
		Environment *environment.Environment
	}{
		State: state{
			BaseName:    stateFileName,
			Path:        st.FilePath,
			AbsPath:     stateAbsPath,
			AbsPathSHA1: sha1sum,
		},
		Release:     release,
		Environment: &st.Env,
	}

	if err := t.Execute(buf, data); err != nil {
		return "", fmt.Errorf("executing output-file template: %w", err)
	}

	return buf.String(), nil
}

func (st *HelmState) ToYaml() (string, error) {
	if result, err := yaml.Marshal(st); err != nil {
		return "", err
	} else {
		return string(result), nil
	}
}

func (st *HelmState) LoadYAMLForEmbedding(release *ReleaseSpec, entries []any, missingFileHandler *string, pathPrefix string) ([]any, error) {
	var result []any

	for _, v := range entries {
		switch t := v.(type) {
		case string:
			var values map[string]any

			paths, skip, err := st.storage().resolveFile(missingFileHandler, "values", pathPrefix+t, st.MissingFileHandlerConfig.resolveFileOptions()...)
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

func (st *HelmState) getOCIChart(release *ReleaseSpec, tempDir string, helm helmexec.Interface, outputDirTemplate string) (*string, error) {
	qualifiedChartName, chartName, chartVersion, err := st.getOCIQualifiedChartName(release, helm)
	if err != nil {
		return nil, err
	}

	if qualifiedChartName == "" {
		return nil, nil
	}

	chartPath, _ := st.getOCIChartPath(tempDir, release, chartName, chartVersion, outputDirTemplate)

	if st.fs.DirectoryExistsAt(chartPath) {
		st.logger.Debugf("chart already exists at %s", chartPath)
	} else {
		flags := st.chartOCIFlags(release)
		flags = st.appendVerifyFlags(flags, release)
		flags = st.appendKeyringFlags(flags, release)
		flags = st.appendChartDownloadFlags(flags, release)
		flags = st.appendChartVersionFlags(flags, release)

		err := helm.ChartPull(qualifiedChartName, chartPath, flags...)
		if err != nil {
			return nil, err
		}

		err = helm.ChartExport(qualifiedChartName, chartPath)
		if err != nil {
			return nil, err
		}
	}

	fullChartPath, err := findChartDirectory(chartPath)
	if err != nil {
		return nil, err
	}

	chartPath = filepath.Dir(fullChartPath)

	return &chartPath, nil
}

// IsOCIChart returns true if the chart is an OCI chart
func (st *HelmState) IsOCIChart(chart string) bool {
	if strings.HasPrefix(chart, "oci://") {
		return true
	}

	repo, _ := st.GetRepositoryAndNameFromChartName(chart)
	if repo == nil {
		return false
	}
	return repo.OCI
}

func (st *HelmState) getOCIQualifiedChartName(release *ReleaseSpec, helm helmexec.Interface) (string, string, string, error) {
	chartVersion := "latest"
	if st.isDevelopment(release) && release.Version == "" {
		// omit version, otherwise --devel flag is ignored by helm and helm-diff
		chartVersion = ""
	} else if release.Version != "" {
		chartVersion = release.Version
	}

	if !st.IsOCIChart(release.Chart) {
		return "", "", chartVersion, nil
	}

	var qualifiedChartName, chartName string
	if strings.HasPrefix(release.Chart, "oci://") {
		parts := strings.Split(release.Chart, "/")
		chartName = parts[len(parts)-1]
		qualifiedChartName = strings.Replace(fmt.Sprintf("%s:%s", release.Chart, chartVersion), "oci://", "", 1)
	} else {
		var repo *RepositorySpec
		repo, chartName = st.GetRepositoryAndNameFromChartName(release.Chart)
		qualifiedChartName = fmt.Sprintf("%s/%s:%s", repo.URL, chartName, chartVersion)
	}
	qualifiedChartName = strings.TrimSuffix(qualifiedChartName, ":")

	if chartVersion == "latest" && helm.IsVersionAtLeast("3.8.0") {
		return "", "", "", fmt.Errorf("the version for OCI charts should be semver compliant, the latest tag is not supported anymore for helm >= 3.8.0")
	}

	return qualifiedChartName, chartName, chartVersion, nil
}

func (st *HelmState) FullFilePath() (string, error) {
	var wd string
	var err error
	if st.fs != nil {
		wd, err = st.fs.Getwd()
	}
	return filepath.Join(wd, st.basePath, st.FilePath), err
}

func (st *HelmState) getOCIChartPath(tempDir string, release *ReleaseSpec, chartName, chartVersion, outputDirTemplate string) (string, error) {
	if outputDirTemplate != "" {
		return generateChartPath(chartName, tempDir, release, outputDirTemplate)
	}

	pathElems := []string{tempDir}

	if release.Namespace != "" {
		pathElems = append(pathElems, release.Namespace)
	}

	if release.KubeContext != "" {
		pathElems = append(pathElems, release.KubeContext)
	}

	pathElems = append(pathElems, release.Name, chartName, safeVersionPath(chartVersion))

	return filepath.Join(pathElems...), nil
}
