package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

func addFetchSubcommand(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cli.Command{
		Name:  "fetch",
		Usage: "fetch charts from state file",
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:  "concurrency",
				Value: 0,
				Usage: "maximum number of concurrent downloads of release charts",
			},
			cli.BoolFlag{
				Name:  "skip-deps",
				Usage: `skip running "helm repo update" and "helm dependency build"`,
			},
			cli.StringFlag{
				Name:  "output-dir",
				Usage: "directory to store charts (default: temporary directory which is deleted when the command terminates)",
			},
		},
		Action: Action(func(a *app.App, c config.ConfigImpl) error {
			return a.Fetch(c)
		}),
	})
}
