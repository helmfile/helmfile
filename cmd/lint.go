package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

func addLintSubcommand(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cli.Command{
		Name:  "lint",
		Usage: "lint charts from state file (helm lint)",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "args",
				Value: "",
				Usage: "pass args to helm exec",
			},
			cli.StringSliceFlag{
				Name:  "set",
				Usage: "additional values to be merged into the command",
			},
			cli.StringSliceFlag{
				Name:  "values",
				Usage: "additional value files to be merged into the command",
			},
			cli.IntFlag{
				Name:  "concurrency",
				Value: 0,
				Usage: "maximum number of concurrent downloads of release charts",
			},
			cli.BoolFlag{
				Name:  "skip-deps",
				Usage: `skip running "helm repo update" and "helm dependency build"`,
			},
		},
		Action: Action(func(a *app.App, c config.ConfigImpl) error {
			return a.Lint(c)
		}),
	})
}
