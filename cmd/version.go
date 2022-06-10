package cmd

import (
	"github.com/urfave/cli"
)

func addVersionSubcommand(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cli.Command{
		Name:      "version",
		Usage:     "Show the version for Helmfile.",
		ArgsUsage: "[command]",
		Action: func(c *cli.Context) error {
			cli.ShowVersion(c)
			return nil
		},
	})
}
