package state

import (
	"regexp"

	"github.com/helmfile/helmfile/pkg/environment"
)

// TemplateSpec defines the structure of a reusable and composable template for helm releases.
type TemplateSpec struct {
	ReleaseSpec `yaml:",inline"`
}

// stateFile is the info about state file
type StateFile struct {
	// Name is the name of the current state file
	Name string
	// BasePath is the base path of the current state file
	BasePath string

	// RootPath is the root path of the state file
	RootPath string
}

type RootStateFile struct {
	// Path is the root path of the state file
	Path string
}

// stateFileInfo is the info about state file
type StateFileInfo struct {
	// Name is the name of the current state file
	StateFile StateFile

	// RootStateFile is the root path of the state file
	RootStateFile RootStateFile
}

// TrimPartInfo removes the part info from the state file name
func (sf *StateFile) TrimPartInfo() {
	sf.Name = regexp.MustCompile(`\.part\.\d+$`).ReplaceAllString(sf.Name, "")
}

// NewStateFile creates a new StateFile
func NewStateFileInfo(currentStateFileName, currentStateFileBasePath, rootStateFilePath string) StateFileInfo {
	csf := StateFile{Name: currentStateFileName, BasePath: currentStateFileBasePath}
	csf.TrimPartInfo()

	rsf := RootStateFile{Path: rootStateFilePath}

	sfi := StateFileInfo{StateFile: csf, RootStateFile: rsf}

	return sfi
}

// EnvironmentTemplateData provides variables accessible while executing golang text/template expressions in helmfile and values YAML files
type EnvironmentTemplateData struct {
	// Environment is accessible as `.Environment` from any template executed by the renderer
	Environment environment.Environment
	// Namespace is accessible as `.Namespace` from any non-values template executed by the renderer
	Namespace string
	// Values is accessible as `.Values` and it contains default state values overrode by environment values and override values.
	Values        map[string]interface{}
	StateFile     StateFile
	RootStateFile RootStateFile
	StateValues   *map[string]interface{}
}

func NewEnvironmentTemplateData(environment environment.Environment, namespace string, values map[string]interface{}, stateFileInfo StateFileInfo) *EnvironmentTemplateData {
	d := EnvironmentTemplateData{environment, namespace, values, stateFileInfo.StateFile, stateFileInfo.RootStateFile, nil}
	d.StateValues = &d.Values
	return &d
}

// releaseTemplateData provides variables accessible while executing golang text/template expressions in release templates
// and release values templates within a Helmfile YAML file.
type releaseTemplateData struct {
	// Environment is accessible as `.Environment` from any template expression executed by the renderer
	Environment environment.Environment
	// Release is accessible as `.Release` from any template expression executed by the renderer.
	// It contains a subset of ReleaseSpec that is known to be useful to dynamically render values.
	Release releaseTemplateDataRelease
	// Values is accessible as `.Values` and it contains default state values overrode by environment values and override values.
	Values        map[string]interface{}
	StateValues   *map[string]interface{}
	StateFile     StateFile
	RootStateFile RootStateFile
	// KubeContext is HelmState.OverrideKubeContext.
	// You should better use Release.KubeContext as it might work as you'd expect even if HelmState.OverrideKubeContext is not set.
	// See releaseTemplateDataRelease.KubeContext for more information.
	KubeContext string
	// Namespace is HelmState.OverrideNamespace.
	// You should better use Release.Namespace as it might work as you'd expect even if OverrideNamespace is not set.
	// See releaseTemplateDataRelease.Namespace for more information.
	Namespace string
	// Chart is HelmState.OverrideChart.
	// You should better use Release.Chart as it might work as you'd expect even if OverrideChart is not set.
	// See releaseTemplateDataRelease.Chart for more information.
	Chart string
}

type releaseTemplateDataRelease struct {
	// Name is basically ReleaseSpec.Name exposed to the template
	Name string

	// Namespace is HelmState.OverrideNamespace, or if it's empty, ReleaseSpec.Namespace.
	Namespace string

	// Labels is ReleaseSpec.Labels
	Labels map[string]string

	// Chart is ReleaseSpec.Chart
	Chart string

	// KubeContext is ReleaseSpec.KubeContext
	KubeContext string
}
