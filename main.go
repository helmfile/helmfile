package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/helmfile/helmfile/cmd"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/errors"
)

func main() {
	var sig os.Signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig = <-sigs
	}()

	globalConfig := new(config.GlobalOptions)
	rootCmd, err := cmd.NewRootCmd(globalConfig, os.Args[1:])
	errors.HandleExitCoder(err)

	if err := rootCmd.Execute(); err != nil {
		if sig != nil {
			fmt.Fprintln(os.Stderr, err)
			// See http://tldp.org/LDP/abs/html/exitcodes.html
			switch sig {
			case syscall.SIGINT:
				os.Exit(130)
			case syscall.SIGTERM:
				os.Exit(143)
			}
		}
		errors.HandleExitCoder(err)
	}
}
