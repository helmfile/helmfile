package plugins

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/helmfile/vals"

	"github.com/helmfile/helmfile/pkg/envvar"
)

const (
	// cache size for improving performance of ref+.* secrets rendering
	valsCacheSize = 512
)

var instance vals.Evaluator
var mu sync.Mutex

// ErrValsDisabled is returned by the evaluator when HELMFILE_DISABLE_VALS_STRICT is set
// and a `ref+` expression is encountered.
var ErrValsDisabled = errors.New("vals is disabled via HELMFILE_DISABLE_VALS_STRICT environment variable")

// refPlusRegexp mirrors the reference syntax understood by the vals library
// (`ref+<provider>://...` and `secretref+<provider>://...`), so that disabled-vals
// modes only report values that vals itself would have tried to resolve.
var refPlusRegexp = regexp.MustCompile(`(secret)?ref\+[^\s+:]*://`)

// passthroughEvaluator passes values through unchanged (for external vals)
type passthroughEvaluator struct{}

func (p *passthroughEvaluator) Eval(m map[string]any) (map[string]any, error) {
	return normalizeMap(m), nil
}

// strictEvaluator passes through values but errors if ref+ is detected
type strictEvaluator struct{}

func (s *strictEvaluator) Eval(m map[string]any) (map[string]any, error) {
	if containsRefPlus(m) {
		return nil, ErrValsDisabled
	}
	return normalizeMap(m), nil
}

// normalizeMap converts []string values to []any to match vals.Eval behavior.
func normalizeMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = normalizeValue(v)
	}
	return out
}

// normalizeValue recursively converts []string to []any and map[any]any to
// map[string]any, matching the type normalization performed by vals.Eval.
func normalizeValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return normalizeMap(typed)
	case map[any]any:
		strmap := make(map[string]any, len(typed))
		for k, v := range typed {
			strmap[fmt.Sprintf("%v", k)] = normalizeValue(v)
		}
		return strmap
	case []any:
		a := make([]any, len(typed))
		for i, e := range typed {
			a[i] = normalizeValue(e)
		}
		return a
	case []string:
		a := make([]any, len(typed))
		for i, s := range typed {
			a[i] = s
		}
		return a
	default:
		return v
	}
}

func containsRefPlus(v any) bool {
	switch val := v.(type) {
	case string:
		return refPlusRegexp.MatchString(val)
	case map[string]any:
		for _, v := range val {
			if containsRefPlus(v) {
				return true
			}
		}
	case map[any]any:
		for _, v := range val {
			if containsRefPlus(v) {
				return true
			}
		}
	case []any:
		for _, v := range val {
			if containsRefPlus(v) {
				return true
			}
		}
	case []string:
		for _, s := range val {
			if refPlusRegexp.MatchString(s) {
				return true
			}
		}
	}
	return false
}

func buildValsOptions() (vals.Options, error) {
	// Configure AWS SDK logging via HELMFILE_AWS_SDK_LOG_LEVEL environment variable
	// Default: "off" to prevent sensitive information (tokens, auth headers) from being exposed
	// See issue #2270 and vals PR helmfile/vals#893
	//
	// Valid values:
	// - "off" (default): No AWS SDK logging - secure, prevents credential leakage
	// - "minimal": Log retries only - minimal debugging info
	// - "standard": Log retries + requests - moderate debugging (previous default)
	// - "verbose": Log everything - full debugging (requests, responses, bodies, signing)
	// - Custom: Comma-separated values like "request,response"
	//
	// Note: AWS_SDK_GO_LOG_LEVEL environment variable always takes precedence over this setting
	// Note: Case-insensitive for known values like "off", "OFF", "Off"
	logLevel := strings.TrimSpace(os.Getenv(envvar.AWSSDKLogLevel))

	// Configure fail on missing key behavior
	// Default to false for backward compatibility
	// Set HELMFILE_VALS_FAIL_ON_MISSING_KEY_IN_MAP=true to enable strict mode
	// Supports common boolean values: "true", "TRUE", "1", etc.
	// See issue #1563
	envVal := strings.TrimSpace(os.Getenv(envvar.ValsFailOnMissingKeyInMap))
	var failOnMissingKey bool
	if envVal != "" {
		var err error
		failOnMissingKey, err = strconv.ParseBool(envVal)
		if err != nil {
			return vals.Options{}, fmt.Errorf("invalid value for %s: %q (must be a valid boolean)", envvar.ValsFailOnMissingKeyInMap, envVal)
		}
	}

	// Default to "off" for security if not specified
	if logLevel == "" {
		logLevel = "off"
	}

	// Normalize known values to lowercase for case-insensitive handling
	if strings.EqualFold(logLevel, "off") {
		logLevel = "off"
	}

	opts := vals.Options{
		CacheSize:             valsCacheSize,
		FailOnMissingKeyInMap: failOnMissingKey,
		AWSLogLevel:           logLevel,
	}

	// Also suppress vals' own internal logging unless user wants verbose output
	// This prevents vals' log messages (separate from AWS SDK logs) from exposing credentials
	if logLevel == "off" {
		opts.LogOutput = io.Discard
	}
	// For other levels, allow vals to log to default output for debugging

	return opts, nil
}

func ValsInstance() (vals.Evaluator, error) {
	mu.Lock()
	defer mu.Unlock()

	if instance != nil {
		return instance, nil
	}

	// HELMFILE_DISABLE_VALS_STRICT: error on ref+ usage
	strict, _ := strconv.ParseBool(os.Getenv(envvar.DisableValsStrict))
	if strict {
		instance = &strictEvaluator{}
		return instance, nil
	}

	// HELMFILE_DISABLE_VALS: pass-through for external vals
	disabled, _ := strconv.ParseBool(os.Getenv(envvar.DisableVals))
	if disabled {
		instance = &passthroughEvaluator{}
		return instance, nil
	}

	opts, err := buildValsOptions()
	if err != nil {
		return nil, err
	}

	instance, err = vals.New(opts)
	if err != nil {
		return nil, err
	}

	return instance, nil
}
