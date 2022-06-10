package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

func addDestroySubcommand(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cli.Command{
		Name:  "destroy",
		Usage: "deletes and then purges releases",
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
			cli.BoolFlag{
				Name:  "skip-deps",
				Usage: `skip running "helm repo update" and "helm dependency build"`,
			},
		},
		Action: Action(func(a *app.App, c config.ConfigImpl) error {
			return a.Destroy(c)
		}),
	})
}
