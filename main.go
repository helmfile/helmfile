package main

import (
	"fmt"
	"os"

	"github.com/helmfile/helmfile/cmd"
)

func main() {

	rootCmd := cmd.RootCommand()
	err := rootCmd.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(3)
	}
}
