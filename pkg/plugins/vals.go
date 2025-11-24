package plugins

import (
	"io"
	"os"
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
var once sync.Once

func ValsInstance() (*vals.Runtime, error) {
	var err error
	once.Do(func() {
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
