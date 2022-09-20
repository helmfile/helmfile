package app

import (
	"os"
	"strings"

	"github.com/helmfile/helmfile/pkg/envvar"
)

const (
	DefaultHelmfile              = "helmfile.yaml"
	DeprecatedHelmfile           = "charts.yaml"
	DefaultHelmfileDirectory     = "helmfile.d"
	ExperimentalSelectorExplicit = "explicit-selector-inheritance" // value to remove default selector inheritance to sub-helmfiles and use the explicit one
	HelmDiffRequiredVersion      = "3.5.0"
)

func experimentalModeEnabled() bool {
	return os.Getenv(envvar.Experimental) == "true"
}

func isExplicitSelectorInheritanceEnabled() bool {
	return experimentalModeEnabled() || strings.Contains(os.Getenv(envvar.Experimental), ExperimentalSelectorExplicit)
}
