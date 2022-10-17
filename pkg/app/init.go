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
	HelmRequiredVersion     = "v2.10.0"
	HelmRecommendedVersion  = "v3.10.1"
	HelmDiffRequiredVersion = "v3.4.0"
	HelmInstallCommand      = "https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3"
)

var manuallyInstallCode = 1

type helmRequiredPlugin struct {
	name    string
	version string
	repo    string
}

type HelmfileInit struct {
	helmBinary string
	logger     *zap.SugaredLogger
	runner     helmexec.Runner
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
	defer func() { _ = resp.Body.Close() }()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func NewHelmfileInit(helmBinary string, logger *zap.SugaredLogger, runner helmexec.Runner) *HelmfileInit {
	return &HelmfileInit{helmBinary: helmBinary, logger: logger, runner: runner}
}
func (h *HelmfileInit) UpdateHelm() error {
	return h.InstallHelm()
}

func (h *HelmfileInit) installHelmOnWindows() error {
	windowPackageManagers := make(map[string]string)
	windowPackageManagers["scoop"] = fmt.Sprintf("scoop install helm@%s", strings.TrimLeft(HelmRecommendedVersion, "v"))
	windowPackageManagers["choco"] = fmt.Sprintf("choco install kubernetes-helm --version %s", strings.TrimLeft(HelmRecommendedVersion, "v"))
	for name, command := range windowPackageManagers {
		_, err := exec.LookPath(name)
		if err == nil {
			askYes := AskForConfirmation(fmt.Sprintf("use: '%s'", command))
			if !askYes {
				return &Error{msg: "cancel automatic installation, please install helm manually", code: &manuallyInstallCode}
			}
			_, err = h.runner.Execute("cmd", []string{
				"/c",
				command,
			}, nil, true)
			if err != nil {
				return err
			}
			return nil
		}
	}

	return &Error{msg: "windows platform, please install helm manually, installation steps: https://helm.sh/docs/intro/install/", code: &manuallyInstallCode}
}

func (h *HelmfileInit) InstallHelm() error {
	if runtime.GOOS == "windows" {
		return h.installHelmOnWindows()
	}
	askYes := AskForConfirmation(fmt.Sprintf("use: '%s'", HelmInstallCommand))
	if !askYes {
		return &Error{msg: "cancel automatic installation, please install helm manually", code: &manuallyInstallCode}
	}
	getHelmScript := "/tmp/get-helm-3.sh"
	err := downloadfile(getHelmScript, HelmInstallCommand)
	if err != nil {
		return err
	}
	_, err = h.runner.Execute("bash", []string{
		getHelmScript,
		"--version",
		HelmRecommendedVersion,
	}, nil, true)
	if err != nil {
		return err
	}
	h.helmBinary = DefaultHelmBinary
	return nil
}

func (h *HelmfileInit) CheckHelmPlugins() error {
	plugins := []helmRequiredPlugin{{
		name:    "diff",
		version: HelmDiffRequiredVersion,
		repo:    "https://github.com/databus23/helm-diff",
	}}
	settings := cli.New()
	helm := helmexec.New(h.helmBinary, false, h.logger, "", h.runner)
	for _, p := range plugins {
		pluginVersion, err := helmexec.GetPluginVersion(p.name, settings.PluginsDirectory)
		if err != nil {
			if !strings.Contains(err.Error(), "not installed") {
				return err
			}
			askYes := AskForConfirmation(fmt.Sprintf("The helm plugin %s is not installed, do you need to install it", p.name))
			if !askYes {
				return &Error{msg: "cancel automatic installation, please install manually", code: &manuallyInstallCode}
			}
			err2 := helm.AddPlugin(p.name, p.repo)
			if err2 != nil {
				return err2
			}
			pluginVersion, _ = helmexec.GetPluginVersion(p.name, settings.PluginsDirectory)
		}
		requiredVersion, _ := semver.NewVersion(p.version)
		if pluginVersion.LessThan(requiredVersion) {
			askYes := AskForConfirmation(fmt.Sprintf("The helm plugin %s version is too low, do you need to update it", p.name))
			if !askYes {
				return &Error{msg: "cancel automatic update, please update manually", code: &manuallyInstallCode}
			}
			err2 := helm.UpdatePlugin(p.name)
			if err2 != nil {
				return err2
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
