package kubedog

import (
	goContext "context"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/util/runtime"
)

var (
	errorHandlersInitOnce sync.Once
)

func init() {
	initErrorHandlers()
}

func initErrorHandlers() {
	errorHandlersInitOnce.Do(func() {
		originalHandlers := runtime.ErrorHandlers
		// nolint: reassign
		runtime.ErrorHandlers = []runtime.ErrorHandler{filterErrorHandler}
		// nolint: reassign
		runtime.ErrorHandlers = append(runtime.ErrorHandlers, originalHandlers...)
	})
}

func filterErrorHandler(ctx goContext.Context, err error, msg string, keysAndValues ...interface{}) {
	if err == nil {
		return
	}

	errMsg := err.Error()

	if strings.Contains(errMsg, "context canceled") {
		return
	}

	if strings.Contains(errMsg, "Client.Timeout exceeded") {
		return
	}

	for _, handler := range runtime.ErrorHandlers[1:] {
		handler(ctx, err, msg, keysAndValues...)
	}
}
