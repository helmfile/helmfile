package plugins

import (
	"fmt"
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

var instance *vals.Runtime
var mu sync.Mutex

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

func ValsInstance() (*vals.Runtime, error) {
	mu.Lock()
	defer mu.Unlock()

	if instance != nil {
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
