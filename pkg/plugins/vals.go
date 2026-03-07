package plugins

import (
	"errors"
	"io"
	"os"
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
var once sync.Once

var ErrValsDisabled = errors.New("vals is disabled via HELMFILE_DISABLE_VALS_STRICT environment variable")

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
		if ss, ok := v.([]string); ok {
			a := make([]any, len(ss))
			for i, s := range ss {
				a[i] = s
			}
			out[k] = a
		} else {
			out[k] = v
		}
	}
	return out
}

func containsRefPlus(v any) bool {
	switch val := v.(type) {
	case string:
		return strings.Contains(val, "ref+")
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
			if strings.Contains(s, "ref+") {
				return true
			}
		}
	}
	return false
}

func ValsInstance() (vals.Evaluator, error) {
	var err error
	once.Do(func() {
		// HELMFILE_DISABLE_VALS_STRICT: error on ref+ usage
		strict, _ := strconv.ParseBool(os.Getenv(envvar.DisableValsStrict))
		if strict {
			instance = &strictEvaluator{}
			return
		}

		// HELMFILE_DISABLE_VALS: pass-through for external vals
		disabled, _ := strconv.ParseBool(os.Getenv(envvar.DisableVals))
		if disabled {
			instance = &passthroughEvaluator{}
			return
		}

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
		logLevel := strings.TrimSpace(os.Getenv(envvar.AWSSDKLogLevel))

		opts := vals.Options{
			CacheSize: valsCacheSize,
		}

		// Default to "off" for security if not specified
		if logLevel == "" {
			logLevel = "off"
		}

		// Set AWS SDK log level for vals library
		opts.AWSLogLevel = logLevel

		// Also suppress vals' own internal logging unless user wants verbose output
		// This prevents vals' log messages (separate from AWS SDK logs) from exposing credentials
		if logLevel == "off" {
			opts.LogOutput = io.Discard
		}
		// For other levels, allow vals to log to default output for debugging

		instance, err = vals.New(opts)
	})

	return instance, err
}
