package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

var cacheInfoSubcommand = cli.Command{
	Name:  "info",
	Usage: "cache info",
	Action: Action(func(a *app.App, c config.ConfigImpl) error {
		return a.ShowCacheDir(c)
	}),
}

var cacheCleanupSubcommand = cli.Command{
	Name:  "cleanup",
	Usage: "clean up cache directory",
	Action: Action(func(a *app.App, c config.ConfigImpl) error {
		return a.CleanCacheDir(c)
	}),
}

func addCacheSubcommand(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cli.Command{
		Name:      "cache",
		Usage:     "cache management",
		ArgsUsage: "[command]",
		Subcommands: []cli.Command{
			cacheCleanupSubcommand,
			cacheInfoSubcommand,
		},
	})
}
