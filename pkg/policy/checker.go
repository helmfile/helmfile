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
	topkeysPriority                              = map[string]int{
		"bases":        0,
		"environments": 1,
		"releases":     2,
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

func isTopOrderKey(item []byte) bool {
	key := strings.Split(string(item), ":")[0]
	for k := range topkeysPriority {
		if k == key {
			return true
		}
	}
	return false
}

// TopConfigKeysVerifier verifies the top-level config keys are defined in the correct order.
func TopConfigKeysVerifier(helmfileContent []byte) error {
	var keys [][]byte
	clines := bytes.Split(helmfileContent, []byte("\n"))

	for _, line := range clines {
		if topConfigKeysRegex.Match(line) && isTopOrderKey(line) {
			keys = append(keys, line)
		}
	}

	if len(keys) == 0 {
		return fmt.Errorf("no top-level config keys are found")
	}

	for i := 1; i < len(keys); i++ {
		preKey := strings.Split(string(keys[i-1]), ":")[0]
		currentKey := strings.Split(string(keys[i]), ":")[0]
		if topkeysPriority[preKey] > topkeysPriority[currentKey] {
			return fmt.Errorf("top-level config key %s must be defined before %s", preKey, currentKey)
		}
	}
	return nil
}
