package kubedog

import (
	"sync"

	"k8s.io/klog/v2"
)

var (
	errorHandlersInitOnce sync.Once
)

func initErrorHandlers() {
	errorHandlersInitOnce.Do(func() {
		klog.SetLogger(klog.Background())
	})
}
