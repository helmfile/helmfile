package main

import (
	"os"

	"github.com/helmfile/helmfile/cmd"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/errors"
)

func main() {
	globalConfig := new(config.GlobalOptions)
	rootCmd, err := cmd.NewRootCmd(globalConfig, os.Args[1:])
	errors.HandleExitCoder(err)

	if err := rootCmd.Execute(); err != nil {
		errors.HandleExitCoder(err)
	}
}
