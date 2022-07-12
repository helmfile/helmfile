package main

import (
	"fmt"
	"os"

	"github.com/helmfile/helmfile/cmd"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

func warning(format string, v ...interface{}) {
	format = fmt.Sprintf("WARNING: %s\n", format)
	fmt.Fprintf(os.Stderr, format, v...)
}

func main() {
	globalConfig := new(config.GlobalOptions)
	rootCmd, err := cmd.NewRootCmd(globalConfig, os.Args[1:])
	if err != nil {
		warning("%+v", err)
		os.Exit(1)
	}
	if err := rootCmd.Execute(); err != nil {
		cli.HandleExitCoder(err)
	}
}
