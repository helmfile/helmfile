package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

func addTestSubcommand(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cli.Command{
		Name:  "test",
		Usage: "test releases from state file (helm test)",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "cleanup",
				Usage: "delete test pods upon completion",
			},
			cli.BoolFlag{
				Name:  "logs",
				Usage: "Dump the logs from test pods (this runs after all tests are complete, but before any cleanup)",
			},
			cli.StringFlag{
				Name:  "args",
				Value: "",
				Usage: "pass additional args to helm exec",
			},
			cli.IntFlag{
				Name:  "timeout",
				Value: 300,
				Usage: "maximum time for tests to run before being considered failed",
			},
			cli.IntFlag{
				Name:  "concurrency",
				Value: 0,
				Usage: "maximum number of concurrent helm processes to run, 0 is unlimited",
			},
			cli.BoolFlag{
				Name:  "skip-deps",
				Usage: `skip running "helm repo update" and "helm dependency build"`,
			},
		},
		Action: Action(func(a *app.App, c config.ConfigImpl) error {
			return a.Test(c)
		}),
	})
}
