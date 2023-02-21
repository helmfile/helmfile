// Package policy provides a policy checker for the helmfile state.
package policy

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/helmfile/helmfile/pkg/runtime"
)

var (
	EnvironmentsAndReleasesWithinSameYamlPartErr = errors.New("environments and releases cannot be defined within the same YAML part. Use --- to extract the environments into a dedicated part")
	topConfigKeysRegex                           = regexp.MustCompile(`^[a-zA-Z]+: *$`)
	topkeysPriority                              = []string{
		"bases",
		"environments",
		"releases",
	}
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

// checkOrderOfTopConfigKeys checks the order of top-level config keys.
func checkOrderOfTopConfigKeys(ix int, key string) error {
	if ix >= len(topkeysPriority) {
		return fmt.Errorf("index %d must be less than %d", ix, len(topkeysPriority))
	}

	found := false
	for _, k := range topkeysPriority {
		if key == k {
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	for i := 0; i <= ix; i++ {
		if topkeysPriority[i] == key {
			return nil
		}
	}
	return fmt.Errorf("top-level config key %s must be defined before %s", topkeysPriority[ix], key)
}

// TopConfigKeysVerifier verifies the top-level config keys are defined in the correct order.
func TopConfigKeysVerifier(helmfileContent []byte) error {
	var keys [][]byte
	clines := bytes.Split(helmfileContent, []byte("\n"))

	for _, line := range clines {
		if topConfigKeysRegex.Match(line) {
			keys = append(keys, line)
		}
	}

	if len(keys) == 0 {
		return fmt.Errorf("no top-level config keys are found")
	}

	for i := 1; i < len(keys); i++ {
		key := strings.Split(string(keys[i]), ":")[0]
		switch key {
		case "bases":
			for _, pk := range keys[:i] {
				pks := strings.Split(string(pk), ":")[0]
				if err := checkOrderOfTopConfigKeys(0, pks); err != nil {
					return fmt.Errorf("bases must be defined at the top of the file")
				}
			}
		case "environments":
			for _, pk := range keys[:i] {
				pks := strings.Split(string(pk), ":")[0]
				if err := checkOrderOfTopConfigKeys(1, pks); err != nil {
					return fmt.Errorf("environments must be defined after bases")
				}
			}
		case "releases":
			for _, pk := range keys[:i] {
				pks := strings.Split(string(pk), ":")[0]
				if err := checkOrderOfTopConfigKeys(2, pks); err != nil {
					return fmt.Errorf("releases must be defined after environments")
				}
			}
		}
	}
	return nil
}
