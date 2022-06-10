package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

func addListSubcommand(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cli.Command{
		Name:  "list",
		Usage: "list releases defined in state file",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "output",
				Value: "",
				Usage: "output releases list as a json string",
			},
			cli.BoolFlag{
				Name:  "keep-temp-dir",
				Usage: "Keep temporary directory",
			},
		},
		Action: Action(func(a *app.App, c config.ConfigImpl) error {
			return a.ListReleases(c)
		}),
	})
}
