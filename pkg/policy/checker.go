// Package policy provides a policy checker for the helmfile state.
package policy

import (
	"errors"
	"path/filepath"

	"github.com/helmfile/helmfile/pkg/runtime"
)

var (
	EnvironmentsAndReleasesWithinSameYamlPartErr = errors.New("environments and releases cannot be defined within the same YAML part. Use --- to extract the environments into a dedicated part")
)

// checkerFunc is a function that checks the helmState.
type checkerFunc func(string, map[string]any) (bool, error)

func forbidEnvironmentsWithReleases(filePath string, releaseState map[string]any) (bool, error) {
	// forbid environments and releases to be defined at the same yaml part
	_, hasEnvironments := releaseState["environments"]
	_, hasReleases := releaseState["releases"]
	if hasEnvironments && hasReleases && (filepath.Ext(filePath) == ".gotmpl" || !runtime.V1Mode) {
		return runtime.V1Mode, EnvironmentsAndReleasesWithinSameYamlPartErr
	}
	return false, nil
}

var checkerFuncs = []checkerFunc{
	forbidEnvironmentsWithReleases,
}

// Checker is a policy checker for the helmfile state.
func Checker(filePath string, helmState map[string]any) (bool, error) {
	for _, fn := range checkerFuncs {
		if isStrict, err := fn(filePath, helmState); err != nil {
			return isStrict, err
		}
	}
	return false, nil
}
