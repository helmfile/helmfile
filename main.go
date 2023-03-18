package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/helmfile/helmfile/cmd"
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/errors"
)

func main() {
	var sig os.Signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig = <-sigs

		if sig != nil {
			app.Cancel()
			app.CleanWaitGroup.Wait()

			// See http://tldp.org/LDP/abs/html/exitcodes.html
			switch sig {
			case syscall.SIGINT:
				os.Exit(130)
			case syscall.SIGTERM:
				os.Exit(143)
			}
		}
	}()

	globalConfig := new(config.GlobalOptions)
	rootCmd, err := cmd.NewRootCmd(globalConfig)
	errors.HandleExitCoder(err)

	if err := rootCmd.Execute(); err != nil {
		errors.HandleExitCoder(err)
	}
}
