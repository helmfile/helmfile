package plugins

import (
	"io"
	"os"
	"sync"

	"github.com/helmfile/vals"
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
		// vals' AWS helper enables SDK request logging by default unless AWS_SDK_GO_LOG_LEVEL is set.
		// Force it off unless the user explicitly opted in to avoid leaking sensitive headers.
		if _, exists := os.LookupEnv("AWS_SDK_GO_LOG_LEVEL"); !exists {
			_ = os.Setenv("AWS_SDK_GO_LOG_LEVEL", "off")
		}

		// Set LogOutput to io.Discard to suppress debug logs from AWS SDK and other providers
		// This prevents sensitive information (tokens, auth headers) from being logged to stdout
		// See issue #2270
		instance, err = vals.New(vals.Options{
			CacheSize: valsCacheSize,
			LogOutput: io.Discard,
		})
	})

	return instance, err
}
