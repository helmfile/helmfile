// Package policy provides a policy checker for the helmfile state.
package policy

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/helmfile/helmfile/pkg/runtime"
)

var (
	EnvironmentsAndReleasesWithinSameYamlPartErr = errors.New("environments and releases cannot be defined within the same YAML part. Use --- to extract the environments into a dedicated part")
	topConfigKeysRegex                           = regexp.MustCompile(`^[a-zA-Z]+:`)
	separatorRegex                               = regexp.MustCompile(`^--- *$`)
)

// checkerFunc is a function that checks the helmState.
type checkerFunc func(filePath string, content []byte) (bool, error)

func forbidEnvironmentsWithReleases(filePath string, content []byte) (bool, error) {
	// forbid environments and releases to be defined at the same yaml part
	topKeys := TopKeys(content, true)
	if len(topKeys) == 0 {
		return true, fmt.Errorf("no top-level config keys are found in %s", filePath)
	}
	result := []string{}
	resultKeys := map[string]interface{}{}
	for _, k := range topKeys {
		if k == "environments" || k == "releases" || k == "---" {
			if _, ok := resultKeys[k]; !ok {
				result = append(result, k)
				if k != "---" {
					resultKeys[k] = nil
				}
			}
		}
	}

	if len(result) < 2 {
		return false, nil
	}
	for i := 0; i < len(result)-1; i++ {
		if result[i] != "---" && result[i+1] != "---" {
			return runtime.V1Mode, EnvironmentsAndReleasesWithinSameYamlPartErr
		}
	}
	return false, nil
}

var checkerFuncs = []checkerFunc{
	forbidEnvironmentsWithReleases,
}

// Checker is a policy checker for the helmfile state.
func Checker(filePath string, content []byte) (bool, error) {
	for _, fn := range checkerFuncs {
		if isStrict, err := fn(filePath, content); err != nil {
			return isStrict, err
		}
	}
	return false, nil
}

// TopKeys returns the top-level config keys.
func TopKeys(helmfileContent []byte, hasSeparator bool) []string {
	var topKeys []string
	clines := bytes.Split(helmfileContent, []byte("\n"))

	for _, line := range clines {
		if topConfigKeysRegex.Match(line) {
			lineStr := strings.Split(string(line), ":")[0]
			topKeys = append(topKeys, lineStr)
		}
		if hasSeparator && separatorRegex.Match(line) {
			topKeys = append(topKeys, strings.TrimSpace(string(line)))
		}
	}
	return topKeys
}
