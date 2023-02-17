package plugins

import (
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
		instance, err = vals.New(vals.Options{CacheSize: valsCacheSize})
	})

	return instance, err
}
