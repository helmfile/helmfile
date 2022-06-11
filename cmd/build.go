package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

func addBuildSubcommand(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cli.Command{
		Name:  "build",
		Usage: "output compiled helmfile state(s) as YAML",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "embed-values",
				Usage: "Read all the values files for every release and embed into the output helmfile.yaml",
			},
		},
		Action: Action(func(a *app.App, c config.ConfigImpl) error {
			return a.PrintState(c)
		}),
	})
}
