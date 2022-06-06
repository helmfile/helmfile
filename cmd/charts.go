package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

func addChartsSubcommand(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cli.Command{
		Name:  "charts",
		Usage: "DEPRECATED: sync releases from state file (helm upgrade --install)",
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
				Usage: "maximum number of concurrent helm processes to run, 0 is unlimited",
			},
		},
		Action: Action(func(a *app.App, c config.ConfigImpl) error {
			return a.DeprecatedSyncCharts(c)
		}),
	})
}
