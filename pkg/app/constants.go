package app

import (
	"os"
	"strings"

	"github.com/helmfile/helmfile/pkg/envvar"
)

const (
	DefaultHelmfile              = "helmfile.yaml"
	DefaultHelmfileDirectory     = "helmfile.d"
	ExperimentalSelectorExplicit = "explicit-selector-inheritance" // value to remove default selector inheritance to sub-helmfiles and use the explicit one
)

func experimentalModeEnabled() bool {
	return os.Getenv(envvar.Experimental) == "true"
}

func isExplicitSelectorInheritanceEnabled() bool {
	return experimentalModeEnabled() || strings.Contains(os.Getenv(envvar.Experimental), ExperimentalSelectorExplicit)
}
