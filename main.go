package main

import (
	gocontext "context"
	"os"
	"os/signal"
	"syscall"

	"github.com/helmfile/helmfile/cmd"
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/errors"
	"github.com/helmfile/helmfile/pkg/telemetry"
)

func main() {
	var sig os.Signal
	sigs := make(chan os.Signal, 1)
	errChan := make(chan error, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		globalConfig := new(config.GlobalOptions)
		rootCmd, err := cmd.NewRootCmd(globalConfig)
		if err != nil {
			errChan <- err
			return
		}

		errChan <- rootCmd.Execute()
	}()

	select {
	case sig = <-sigs:
		if sig != nil {
			app.Cancel()
			app.CleanWaitGroup.Wait()

			shutdownTelemetry(nil, signalExitCode(sig))

			// See http://tldp.org/LDP/abs/html/exitcodes.html
			switch sig {
			case syscall.SIGINT:
				os.Exit(130)
			case syscall.SIGTERM:
				os.Exit(143)
			}
		}
	case err := <-errChan:
		shutdownTelemetry(err, exitCodeOf(err))
		errors.HandleExitCoder(err)
	}
}

// shutdownTelemetry flushes buffered spans before the process exits. It is a
// no-op when tracing was never enabled. Flush failures are deliberately
// ignored: telemetry must not mask the command's own result.
func shutdownTelemetry(runErr error, exitCode int) {
	ctx, cancel := gocontext.WithTimeout(gocontext.Background(), telemetry.ShutdownTimeout)
	defer cancel()
	_ = telemetry.Shutdown(ctx, runErr, exitCode)
}

// signalExitCode maps termination signals to conventional exit codes, matching
// the os.Exit calls in main.
func signalExitCode(sig os.Signal) int {
	switch sig {
	case syscall.SIGINT:
		return 130
	case syscall.SIGTERM:
		return 143
	default:
		return 1
	}
}

// exitCodeOf extracts the exit code from the command result for the root-span
// attribute; success is 0.
func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(errors.ExitCoder); ok {
		return exitErr.ExitCode()
	}
	return 1
}
