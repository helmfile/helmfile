package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

func addStatusSubcommand(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cli.Command{
		Name:  "status",
		Usage: "retrieve status of releases in state file",
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:  "concurrency",
				Value: 0,
				Usage: "maximum number of concurrent helm processes to run, 0 is unlimited",
			},
			cli.StringFlag{
				Name:  "args",
				Value: "",
				Usage: "pass args to helm exec",
			},
		},
		Action: Action(func(a *app.App, c config.ConfigImpl) error {
			return a.Status(c)
		}),
	})
}
