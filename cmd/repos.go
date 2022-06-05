package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

func addReposSubcommand(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cli.Command{
		Name:  "repos",
		Usage: "sync repositories from state file (helm repo add && helm repo update)",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "args",
				Value: "",
				Usage: "pass args to helm exec",
			},
		},
		Action: Action(func(a *app.App, c config.ConfigImpl) error {
			return a.Repos(c)
		}),
	})
}
