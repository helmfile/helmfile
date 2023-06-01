package helmexec

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/helmfile/chartify"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/plugin"

	"github.com/helmfile/helmfile/pkg/yaml"
)

type decryptedSecret struct {
	mutex sync.RWMutex
	bytes []byte
	err   error
}

type HelmExecOptions struct {
	EnableLiveOutput   bool
	DisableForceUpdate bool
}

type execer struct {
	helmBinary           string
	options              HelmExecOptions
	version              *semver.Version
	runner               Runner
	logger               *zap.SugaredLogger
	kubeContext          string
	extra                []string
	decryptedSecretMutex sync.Mutex
	decryptedSecrets     map[string]*decryptedSecret
	writeTempFile        func([]byte) (string, error)
}

func NewLogger(writer io.Writer, logLevel string) *zap.SugaredLogger {
	var cfg zapcore.EncoderConfig
	cfg.MessageKey = "message"
	out := zapcore.AddSync(writer)
	var level zapcore.Level
	err := level.Set(logLevel)
	if err != nil {
		panic(err)
	}
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		out,
		level,
	)
	return zap.New(core).Sugar()
}

func parseHelmVersion(versionStr string) (*semver.Version, error) {
	if len(versionStr) == 0 {
		return nil, fmt.Errorf("empty helm version")
	}

	v, err := chartify.FindSemVerInfo(versionStr)

	if err != nil {
		return nil, fmt.Errorf("error find helm srmver version '%s': %w", versionStr, err)
	}

	ver, err := semver.NewVersion(v)
	if err != nil {
		return nil, fmt.Errorf("error parsing helm version '%s'", versionStr)
	}

	return ver, nil
}

func GetHelmVersion(helmBinary string, runner Runner) (*semver.Version, error) {
	// Autodetect from `helm version`
	outBytes, err := runner.Execute(helmBinary, []string{"version", "--client", "--short"}, nil, false)
	if err != nil {
		return nil, fmt.Errorf("error determining helm version: %w", err)
	}

	return parseHelmVersion(string(outBytes))
}

func GetPluginVersion(name, pluginsDir string) (*semver.Version, error) {
	plugins, err := plugin.FindPlugins(pluginsDir)
	if err != nil {
		return nil, err
	}
	for _, plugin := range plugins {
		if plugin.Metadata.Name == name {
			return semver.NewVersion(plugin.Metadata.Version)
		}
	}

	return nil, fmt.Errorf("plugin %s not installed", name)
}

func redactedURL(chart string) string {
	chartURL, err := url.ParseRequestURI(chart)
	if err != nil {
		return chart
	}
	return chartURL.Redacted()
}

// New for running helm commands
func New(helmBinary string, options HelmExecOptions, logger *zap.SugaredLogger, kubeContext string, runner Runner) *execer {
	// TODO: proper error handling
	version, err := GetHelmVersion(helmBinary, runner)
	if err != nil {
		panic(err)
	}
	return &execer{
		helmBinary:       helmBinary,
		options:          options,
		version:          version,
		logger:           logger,
		kubeContext:      kubeContext,
		runner:           runner,
		decryptedSecrets: make(map[string]*decryptedSecret),
	}
}

func (helm *execer) SetExtraArgs(args ...string) {
	helm.extra = args
}

func (helm *execer) SetHelmBinary(bin string) {
	helm.helmBinary = bin
}

func (helm *execer) SetEnableLiveOutput(enableLiveOutput bool) {
	helm.options.EnableLiveOutput = enableLiveOutput
}

func (helm *execer) SetDisableForceUpdate(forceUpdate bool) {
	helm.options.DisableForceUpdate = forceUpdate
}

func (helm *execer) AddRepo(name, repository, cafile, certfile, keyfile, username, password string, managed string, passCredentials, skipTLSVerify bool) error {
	var args []string
	var out []byte
	var err error
	if name == "" && repository != "" {
		helm.logger.Infof("empty field name\n")
		return fmt.Errorf("empty field name")
	}
	switch managed {
	case "acr":
		helm.logger.Infof("Adding repo %v (acr)", name)
		out, err = helm.azcli(name)
	case "":
		args = append(args, "repo", "add", name, repository)

		// See https://github.com/helm/helm/pull/8777
		if cons, err := semver.NewConstraint(">= 3.3.2"); err == nil {
			if !helm.options.DisableForceUpdate && cons.Check(helm.version) {
				args = append(args, "--force-update")
			}
		} else {
			panic(err)
		}

		if certfile != "" && keyfile != "" {
			args = append(args, "--cert-file", certfile, "--key-file", keyfile)
		}
		if cafile != "" {
			args = append(args, "--ca-file", cafile)
		}
		if username != "" && password != "" {
			args = append(args, "--username", username, "--password", password)
		}
		if passCredentials {
			args = append(args, "--pass-credentials")
		}
		if skipTLSVerify {
			args = append(args, "--insecure-skip-tls-verify")
		}
		helm.logger.Infof("Adding repo %v %v", name, repository)
		out, err = helm.exec(args, map[string]string{}, nil)
	default:
		helm.logger.Errorf("ERROR: unknown type '%v' for repository %v", managed, name)
		out = nil
		err = nil
	}
	helm.info(out)
	return err
}

func (helm *execer) UpdateRepo() error {
	helm.logger.Info("Updating repo")
	out, err := helm.exec([]string{"repo", "update"}, map[string]string{}, nil)
	helm.info(out)
	return err
}

func (helm *execer) RegistryLogin(repository string, username string, password string) error {
	helm.logger.Info("Logging in to registry")
	args := []string{
		"registry",
		"login",
		repository,
		"--username",
		username,
		"--password-stdin",
	}
	buffer := bytes.Buffer{}
	buffer.Write([]byte(fmt.Sprintf("%s\n", password)))
	out, err := helm.execStdIn(args, map[string]string{"HELM_EXPERIMENTAL_OCI": "1"}, &buffer)
	helm.info(out)
	return err
}

func (helm *execer) BuildDeps(name, chart string, flags ...string) error {
	helm.logger.Infof("Building dependency release=%v, chart=%v", name, chart)
	args := []string{
		"dependency",
		"build",
		chart,
	}

	args = append(args, flags...)

	out, err := helm.exec(args, map[string]string{}, nil)
	helm.info(out)
	return err
}

func (helm *execer) UpdateDeps(chart string) error {
	helm.logger.Infof("Updating dependency %v", chart)
	out, err := helm.exec([]string{"dependency", "update", chart}, map[string]string{}, nil)
	helm.info(out)
	return err
}

func (helm *execer) SyncRelease(context HelmContext, name, chart string, flags ...string) error {
	helm.logger.Infof("Upgrading release=%v, chart=%v", name, redactedURL(chart))
	preArgs := make([]string, 0)
	env := make(map[string]string)

	flags = append(flags, "--history-max", strconv.Itoa(context.HistoryMax))

	out, err := helm.exec(append(append(preArgs, "upgrade", "--install", name, chart), flags...), env, nil)
	helm.write(nil, out)
	return err
}

func (helm *execer) ReleaseStatus(context HelmContext, name string, flags ...string) error {
	helm.logger.Infof("Getting status %v", name)
	preArgs := make([]string, 0)
	env := make(map[string]string)
	out, err := helm.exec(append(append(preArgs, "status", name), flags...), env, nil)
	helm.write(nil, out)
	return err
}

func (helm *execer) List(context HelmContext, filter string, flags ...string) (string, error) {
	helm.logger.Infof("Listing releases matching %v", filter)
	preArgs := make([]string, 0)
	env := make(map[string]string)
	args := []string{"list", "--filter", filter}

	enableLiveOutput := false
	out, err := helm.exec(append(append(preArgs, args...), flags...), env, &enableLiveOutput)
	// In v2 we have been expecting `helm list FILTER` prints nothing.
	// In v3 helm still prints the header like `NAME	NAMESPACE	REVISION	UPDATED	STATUS	CHART	APP VERSION`,
	// which confuses helmfile's existing logic that treats any non-empty output from `helm list` is considered as the indication
	// of the release to exist.
	//
	// This fixes it by removing the header from the v3 output, so that the output is formatted the same as that of v2.
	lines := strings.Split(string(out), "\n")
	lines = lines[1:]
	out = []byte(strings.Join(lines, "\n"))
	helm.write(nil, out)
	return string(out), err
}

func (helm *execer) DecryptSecret(context HelmContext, name string, flags ...string) (string, error) {
	absPath, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}

	helm.logger.Debugf("Preparing to decrypt secret %v", absPath)
	helm.decryptedSecretMutex.Lock()

	secret, ok := helm.decryptedSecrets[absPath]

	// Cache miss
	if !ok {
		secret = &decryptedSecret{}
		helm.decryptedSecrets[absPath] = secret

		secret.mutex.Lock()
		defer secret.mutex.Unlock()
		helm.decryptedSecretMutex.Unlock()

		helm.logger.Infof("Decrypting secret %v", absPath)
		preArgs := make([]string, 0)
		env := make(map[string]string)
		settings := cli.New()
		pluginVersion, err := GetPluginVersion("secrets", settings.PluginsDirectory)
		if err != nil {
			secret.err = err
			return "", err
		}
		secretArg := "view"
		// helm secret view command. The helm secret decrypt command is a drop-in replacement in 4.0.0 version
		if pluginVersion.Major() > 3 {
			secretArg = "decrypt"
		}
		enableLiveOutput := false
		secretBytes, err := helm.exec(append(append(preArgs, "secrets", secretArg, absPath), flags...), env, &enableLiveOutput)
		if err != nil {
			secret.err = err
			return "", err
		}

		secret.bytes = secretBytes
	} else {
		// Cache hit
		helm.logger.Debugf("Found secret in cache %v", absPath)

		secret.mutex.RLock()
		helm.decryptedSecretMutex.Unlock()
		defer secret.mutex.RUnlock()

		if secret.err != nil {
			return "", secret.err
		}
	}

	tempFile := helm.writeTempFile

	if tempFile == nil {
		tempFile = func(content []byte) (string, error) {
			dir := filepath.Dir(name)
			extension := filepath.Ext(name)
			tmpFile, err := os.CreateTemp(dir, "secret*"+extension)
			if err != nil {
				return "", err
			}
			defer func() {
				_ = tmpFile.Close()
			}()

			_, err = tmpFile.Write(content)
			if err != nil {
				return "", err
			}

			return tmpFile.Name(), nil
		}
	}

	tmpFileName, err := tempFile(secret.bytes)
	if err != nil {
		return "", err
	}

	helm.logger.Debugf("Decrypted %s into %s", absPath, tmpFileName)

	return tmpFileName, err
}

func (helm *execer) TemplateRelease(name string, chart string, flags ...string) error {
	helm.logger.Infof("Templating release=%v, chart=%v", name, redactedURL(chart))
	args := []string{"template", name, chart}

	out, err := helm.exec(append(args, flags...), map[string]string{}, nil)

	var outputToFile bool

	for _, f := range flags {
		if strings.HasPrefix("--output-dir", f) {
			outputToFile = true
			break
		}
	}

	if outputToFile {
		// With --output-dir is passed to helm-template,
		// we can safely direct all the logs from it to our logger.
		//
		// It's safe because anything written to stdout by helm-template with output-dir is logs,
		// like excessive `wrote path/to/output/dir/chart/template/file.yaml` messages,
		// but manifets.
		//
		// See https://github.com/roboll/helmfile/pull/1691#issuecomment-805636021 for more information.
		helm.info(out)
	} else {
		// Always write to stdout for use with e.g. `helmfile template | kubectl apply -f -`
		helm.write(nil, out)
	}

	return err
}

func (helm *execer) DiffRelease(context HelmContext, name, chart string, suppressDiff bool, flags ...string) error {
	if context.Writer != nil {
		fmt.Fprintf(context.Writer, "Comparing release=%v, chart=%v\n", name, redactedURL(chart))
	} else {
		helm.logger.Infof("Comparing release=%v, chart=%v", name, redactedURL(chart))
	}
	preArgs := make([]string, 0)
	env := make(map[string]string)
	var overrideEnableLiveOutput *bool = nil
	if suppressDiff {
		enableLiveOutput := false
		overrideEnableLiveOutput = &enableLiveOutput
	}

	out, err := helm.exec(append(append(preArgs, "diff", "upgrade", "--allow-unreleased", name, chart), flags...), env, overrideEnableLiveOutput)
	// Do our best to write STDOUT only when diff existed
	// Unfortunately, this works only when you run helmfile with `--detailed-exitcode`
	detailedExitcodeEnabled := false
	for _, f := range flags {
		if strings.Contains(f, "detailed-exitcode") {
			detailedExitcodeEnabled = true
			break
		}
	}
	if detailedExitcodeEnabled {
		e, ok := err.(ExitError)
		if ok && e.ExitStatus() == 2 {
			if !(suppressDiff) {
				helm.write(context.Writer, out)
			}
			return err
		}
	} else if !(suppressDiff) {
		helm.write(context.Writer, out)
	}
	return err
}

func (helm *execer) Lint(name, chart string, flags ...string) error {
	helm.logger.Infof("Linting release=%v, chart=%v", name, chart)
	out, err := helm.exec(append([]string{"lint", chart}, flags...), map[string]string{}, nil)
	helm.write(nil, out)
	return err
}

func (helm *execer) Fetch(chart string, flags ...string) error {
	helm.logger.Infof("Fetching %v", redactedURL(chart))
	out, err := helm.exec(append([]string{"fetch", chart}, flags...), map[string]string{}, nil)
	helm.info(out)
	return err
}

func (helm *execer) ChartPull(chart string, path string, flags ...string) error {
	var helmArgs []string
	helm.logger.Infof("Pulling %v", chart)
	helmVersionConstraint, _ := semver.NewConstraint(">= 3.7.0")
	if helmVersionConstraint.Check(helm.version) {
		// in the 3.7.0 version, the chart pull has been replaced with helm pull
		// https://github.com/helm/helm/releases/tag/v3.7.0
		ociChartURL, ociChartTag := resolveOciChart(chart)
		helmArgs = []string{"pull", ociChartURL, "--version", ociChartTag, "--destination", path, "--untar"}
	} else {
		helmArgs = []string{"chart", "pull", chart}
	}
	out, err := helm.exec(append(helmArgs, flags...), map[string]string{"HELM_EXPERIMENTAL_OCI": "1"}, nil)
	helm.info(out)
	return err
}

func (helm *execer) ChartExport(chart string, path string, flags ...string) error {
	helmVersionConstraint, _ := semver.NewConstraint(">= 3.7.0")
	if helmVersionConstraint.Check(helm.version) {
		// in the 3.7.0 version, the chart export has been removed
		// https://github.com/helm/helm/releases/tag/v3.7.0
		return nil
	}
	var helmArgs []string
	helm.logger.Infof("Exporting %v", chart)
	helmArgs = []string{"chart", "export", chart, "--destination", path}
	out, err := helm.exec(append(helmArgs, flags...), map[string]string{"HELM_EXPERIMENTAL_OCI": "1"}, nil)
	helm.info(out)
	return err
}

func (helm *execer) DeleteRelease(context HelmContext, name string, flags ...string) error {
	helm.logger.Infof("Deleting %v", name)
	preArgs := make([]string, 0)
	env := make(map[string]string)
	out, err := helm.exec(append(append(preArgs, "delete", name), flags...), env, nil)
	helm.write(nil, out)
	return err
}

func (helm *execer) TestRelease(context HelmContext, name string, flags ...string) error {
	helm.logger.Infof("Testing %v", name)
	preArgs := make([]string, 0)
	env := make(map[string]string)
	args := []string{"test", name}
	out, err := helm.exec(append(append(preArgs, args...), flags...), env, nil)
	helm.write(nil, out)
	return err
}

func (helm *execer) AddPlugin(name, path, version string) error {
	helm.logger.Infof("Install helm plugin %v", name)
	out, err := helm.exec([]string{"plugin", "install", path, "--version", version}, map[string]string{}, nil)
	helm.info(out)
	return err
}

func (helm *execer) UpdatePlugin(name string) error {
	helm.logger.Infof("Update helm plugin %v", name)
	out, err := helm.exec([]string{"plugin", "update", name}, map[string]string{}, nil)
	helm.info(out)
	return err
}

func (helm *execer) exec(args []string, env map[string]string, overrideEnableLiveOutput *bool) ([]byte, error) {
	cmdargs := args
	if len(helm.extra) > 0 {
		cmdargs = append(cmdargs, helm.extra...)
	}
	if helm.kubeContext != "" {
		cmdargs = append([]string{"--kube-context", helm.kubeContext}, cmdargs...)
	}
	cmd := fmt.Sprintf("exec: %s %s", helm.helmBinary, strings.Join(cmdargs, " "))
	helm.logger.Debug(cmd)
	enableLiveOutput := helm.options.EnableLiveOutput
	if overrideEnableLiveOutput != nil {
		enableLiveOutput = *overrideEnableLiveOutput
	}
	outBytes, err := helm.runner.Execute(helm.helmBinary, cmdargs, env, enableLiveOutput)
	return outBytes, err
}

func (helm *execer) execStdIn(args []string, env map[string]string, stdin io.Reader) ([]byte, error) {
	cmdargs := args
	if len(helm.extra) > 0 {
		cmdargs = append(cmdargs, helm.extra...)
	}
	if helm.kubeContext != "" {
		cmdargs = append([]string{"--kube-context", helm.kubeContext}, cmdargs...)
	}
	cmd := fmt.Sprintf("exec: %s %s", helm.helmBinary, strings.Join(cmdargs, " "))
	helm.logger.Debug(cmd)
	outBytes, err := helm.runner.ExecuteStdIn(helm.helmBinary, cmdargs, env, stdin)
	return outBytes, err
}

func (helm *execer) azcli(name string) ([]byte, error) {
	cmdargs := append(strings.Split("acr helm repo add --name", " "), name)
	cmd := fmt.Sprintf("exec: az %s", strings.Join(cmdargs, " "))
	helm.logger.Debug(cmd)
	outBytes, err := helm.runner.Execute("az", cmdargs, map[string]string{}, false)
	helm.logger.Debugf("%s: %s", cmd, outBytes)
	return outBytes, err
}

func (helm *execer) info(out []byte) {
	if len(out) > 0 {
		helm.logger.Infof("%s", out)
	}
}

func (helm *execer) write(w io.Writer, out []byte) {
	if len(out) > 0 {
		if w == nil {
			w = os.Stdout
		}
		fmt.Fprintf(w, "%s\n", out)
	}
}

func (helm *execer) IsHelm3() bool {
	return helm.version.Major() == 3
}

func (helm *execer) GetVersion() Version {
	return Version{
		Major: int(helm.version.Major()),
		Minor: int(helm.version.Minor()),
		Patch: int(helm.version.Patch()),
	}
}

func (helm *execer) IsVersionAtLeast(versionStr string) bool {
	ver := semver.MustParse(versionStr)
	return helm.version.Equal(ver) || helm.version.GreaterThan(ver)
}

func resolveOciChart(ociChart string) (ociChartURL, ociChartTag string) {
	var urlTagIndex int
	// Get the last : index
	// e.g.,
	// 1. registry:443/helm-charts
	// 2. registry/helm-charts:latest
	// 3. registry:443/helm-charts:latest
	if strings.LastIndex(ociChart, ":") <= strings.LastIndex(ociChart, "/") {
		urlTagIndex = len(ociChart)
		ociChartTag = ""
	} else {
		urlTagIndex = strings.LastIndex(ociChart, ":")
		ociChartTag = ociChart[urlTagIndex+1:]
	}
	ociChartURL = fmt.Sprintf("oci://%s", ociChart[:urlTagIndex])
	return ociChartURL, ociChartTag
}

func (helm *execer) ShowChart(chartPath string) (chart.Metadata, error) {
	var helmArgs = []string{"show", "chart", chartPath}
	out, error := helm.exec(helmArgs, map[string]string{}, nil)
	if error != nil {
		return chart.Metadata{}, error
	}
	var metadata chart.Metadata
	error = yaml.Unmarshal(out, &metadata)
	if error != nil {
		return chart.Metadata{}, error
	}
	return metadata, nil
}
