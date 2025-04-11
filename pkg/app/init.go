package app

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/cli"

	"github.com/helmfile/helmfile/pkg/helmexec"
)

const (
	HelmRequiredVersion           = "v3.16.4"
	HelmRecommendedVersion        = "v3.17.3"
	HelmDiffRecommendedVersion    = "v3.11.0"
	HelmSecretsRecommendedVersion = "v4.6.3"
	HelmGitRecommendedVersion     = "v1.3.0"
	HelmS3RecommendedVersion      = "v0.16.3"
	HelmInstallCommand            = "https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3"
)

var (
	manuallyInstallCode   = 1
	windowPackageManagers = map[string]string{
		"scoop": fmt.Sprintf("scoop install helm@%s", strings.TrimLeft(HelmRecommendedVersion, "v")),
		"choco": fmt.Sprintf("choco install kubernetes-helm --version %s", strings.TrimLeft(HelmRecommendedVersion, "v")),
	}
	helmPlugins = []helmRecommendedPlugin{
		{
			name:    "diff",
			version: HelmDiffRecommendedVersion,
			repo:    "https://github.com/databus23/helm-diff",
		},
		{
			name:    "secrets",
			version: HelmSecretsRecommendedVersion,
			repo:    "https://github.com/jkroepke/helm-secrets",
		},
		{
			name:    "s3",
			version: HelmS3RecommendedVersion,
			repo:    "https://github.com/hypnoglow/helm-s3.git",
		},
		{
			name:    "helm-git",
			version: HelmGitRecommendedVersion,
			repo:    "https://github.com/aslafy-z/helm-git.git",
		},
	}
)

type helmRecommendedPlugin struct {
	name    string
	version string
	repo    string
}

type HelmfileInit struct {
	helmBinary     string
	configProvider InitConfigProvider
	logger         *zap.SugaredLogger
	runner         helmexec.Runner
}

func downloadfile(filepath string, url string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("download %s error, code: %d", url, resp.StatusCode)
	}
	defer func() { _ = resp.Body.Close() }()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func NewHelmfileInit(helmBinary string, c InitConfigProvider, logger *zap.SugaredLogger, runner helmexec.Runner) *HelmfileInit {
	return &HelmfileInit{helmBinary: helmBinary, configProvider: c, logger: logger, runner: runner}
}
func (h *HelmfileInit) UpdateHelm() error {
	return h.InstallHelm()
}

func (h *HelmfileInit) installHelmOnWindows() error {
	for name, command := range windowPackageManagers {
		_, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		err = h.WhetherContinue(fmt.Sprintf("use: '%s'", command))
		if err != nil {
			return err
		}
		_, err = h.runner.Execute("cmd", []string{
			"/c",
			command,
		}, nil, true)
		return err
	}

	return &Error{msg: "windows platform, please install helm manually, installation steps: https://helm.sh/docs/intro/install/", code: &manuallyInstallCode}
}

func (h *HelmfileInit) InstallHelm() error {
	if runtime.GOOS == "windows" {
		return h.installHelmOnWindows()
	}

	err := h.WhetherContinue(fmt.Sprintf("use: '%s'", HelmInstallCommand))
	if err != nil {
		return err
	}
	getHelmScript, err := os.CreateTemp("", "get-helm-3.sh")
	defer func() {
		_ = getHelmScript.Close()
		_ = os.Remove(getHelmScript.Name())
	}()
	if err != nil {
		return err
	}
	err = downloadfile(getHelmScript.Name(), HelmInstallCommand)
	if err != nil {
		return err
	}
	_, err = h.runner.Execute("bash", []string{
		getHelmScript.Name(),
		"--version",
		HelmRecommendedVersion,
	}, nil, true)
	if err != nil {
		return err
	}
	h.helmBinary = DefaultHelmBinary
	return nil
}

func (h *HelmfileInit) WhetherContinue(ask string) error {
	if h.configProvider.Force() {
		return nil
	}
	askYes := AskForConfirmation(ask)
	if !askYes {
		return &Error{msg: "cancel automatic installation, please install manually", code: &manuallyInstallCode}
	}
	return nil
}

func (h *HelmfileInit) CheckHelmPlugins() error {
	settings := cli.New()
	helm := helmexec.New(h.helmBinary, helmexec.HelmExecOptions{}, h.logger, "", "", h.runner)
	for _, p := range helmPlugins {
		pluginVersion, err := helmexec.GetPluginVersion(p.name, settings.PluginsDirectory)
		if err != nil {
			if !strings.Contains(err.Error(), "not installed") {
				return err
			}

			err = h.WhetherContinue(fmt.Sprintf("The helm plugin %q is not installed, do you want to install it?", p.name))
			if err != nil {
				return err
			}

			err = helm.AddPlugin(p.name, p.repo, p.version)
			if err != nil {
				return err
			}
			pluginVersion, _ = helmexec.GetPluginVersion(p.name, settings.PluginsDirectory)
		}
		requiredVersion, _ := semver.NewVersion(p.version)
		if pluginVersion.LessThan(requiredVersion) {
			err = h.WhetherContinue(fmt.Sprintf("The helm plugin %q version is too low, do you want to update it?", p.name))
			if err != nil {
				return err
			}
			err = helm.UpdatePlugin(p.name)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *HelmfileInit) CheckHelm() error {
	helmExits := true
	_, err := exec.LookPath(h.helmBinary)
	if err != nil {
		helmExits = false
	}
	if !helmExits {
		h.logger.Info("helm not found, needs to be installed")
		err = h.InstallHelm()
		if err != nil {
			return err
		}
	}
	helmversion, err := helmexec.GetHelmVersion(h.helmBinary, h.runner)
	if err != nil {
		return err
	}
	requiredHelmVersion, _ := semver.NewVersion(HelmRequiredVersion)
	if helmversion.LessThan(requiredHelmVersion) {
		h.logger.Infof("helm version is too low, the current version is %s, the required version is %s", helmversion, requiredHelmVersion)
		err = h.UpdateHelm()
		if err != nil {
			return err
		}
	}
	return nil
}
func (h *HelmfileInit) Initialize() error {
	err := h.CheckHelm()
	if err != nil {
		return err
	}
	err = h.CheckHelmPlugins()
	if err != nil {
		return err
	}
	h.logger.Info("helmfile initialization completed!")
	return nil
}
