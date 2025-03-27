// Package policy provides a policy checker for the helmfile state.
package policy

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

var (
	ErrEnvironmentsAndReleasesWithinSameYamlPart = errors.New("environments and releases cannot be defined within the same YAML part. Use --- to extract the environments into a dedicated part")
	topConfigKeysRegex                           = regexp.MustCompile(`^[a-zA-Z]+:`)
	separatorRegex                               = regexp.MustCompile(`^--- *$`)
	topkeysPriority                              = map[string]int{
		"bases":        0,
		"environments": 0,
		"releases":     1,
	}
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
		if slices.Contains([]string{"environments", "releases", "---"}, k) {
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
			return true, ErrEnvironmentsAndReleasesWithinSameYamlPart
		}
	}
	return false, nil
}

var checkerFuncs = []checkerFunc{
	TopConfigKeysVerifier,
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

// isTopOrderKey checks if the key is a top-level config key that must be defined in the correct order.
func isTopOrderKey(key string) bool {
	_, ok := topkeysPriority[key]
	return ok
}

// TopKeys returns the top-level config keys.
func TopKeys(helmfileContent []byte, hasSeparator bool) []string {
	var topKeys []string
	clines := bytes.Split(helmfileContent, []byte("\n"))

	for _, line := range clines {
		lineStr := strings.TrimRightFunc(string(line), unicode.IsSpace)
		if lineStr == "" {
			continue // Skip empty lines
		}
		if hasSeparator && separatorRegex.MatchString(lineStr) {
			topKeys = append(topKeys, lineStr)
		}

		if topConfigKeysRegex.MatchString(lineStr) {
			topKey := strings.SplitN(lineStr, ":", 2)[0]
			topKeys = append(topKeys, topKey)
		}
	}
	return topKeys
}

// TopConfigKeysVerifier verifies the top-level config keys are defined in the correct order.
func TopConfigKeysVerifier(filePath string, helmfileContent []byte) (bool, error) {
	var orderKeys, topKeys []string
	topKeys = TopKeys(helmfileContent, false)

	for _, k := range topKeys {
		if isTopOrderKey(k) {
			orderKeys = append(orderKeys, k)
		}
	}

	if len(topKeys) == 0 {
		return true, fmt.Errorf("no top-level config keys are found in %s", filePath)
	}

	if len(orderKeys) == 0 {
		return false, nil
	}

	for i := 1; i < len(orderKeys); i++ {
		preKey := orderKeys[i-1]
		currentKey := orderKeys[i]
		if topkeysPriority[preKey] > topkeysPriority[currentKey] {
			return true, fmt.Errorf("top-level config key %s must be defined before %s in %s", currentKey, preKey, filePath)
		}
	}
	return false, nil
}
