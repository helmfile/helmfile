package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

func addWriteValuesSubcommand(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cli.Command{
		Name:  "write-values",
		Usage: "write values files for releases. Similar to `helmfile template`, write values files instead of manifests.",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  "set",
				Usage: "additional values to be merged into the command",
			},
			cli.StringSliceFlag{
				Name:  "values",
				Usage: "additional value files to be merged into the command",
			},
			cli.StringFlag{
				Name:  "output-file-template",
				Usage: "go text template for generating the output file. Default: {{ .State.BaseName }}-{{ .State.AbsPathSHA1 }}/{{ .Release.Name}}.yaml",
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
			return a.WriteValues(c)
		}),
	})
}
