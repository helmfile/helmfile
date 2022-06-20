package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

func addSyncSubcommand(cliApp *cli.App) {
	cliApp.Commands = append(cliApp.Commands, cli.Command{
		Name:  "sync",
		Usage: "sync all resources from state file (repos, releases and chart deps)",
		Flags: []cli.Flag{
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
			cli.StringFlag{
				Name:  "args",
				Value: "",
				Usage: "pass args to helm exec",
			},
			cli.BoolFlag{
				Name:  "skip-crds",
				Usage: "if set, no CRDs will be installed on sync. By default, CRDs are installed if not already present",
			},
			cli.BoolFlag{
				Name:  "skip-deps",
				Usage: `skip running "helm repo update" and "helm dependency build"`,
			},
			cli.BoolTFlag{
				Name:  "skip-needs",
				Usage: `do not automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when when --selector/-l flag is not provided. Defaults to true when --include-needs or --include-transitive-needs is not provided`,
			},
			cli.BoolFlag{
				Name:  "include-needs",
				Usage: `automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when when --selector/-l flag is not provided`,
			},
			cli.BoolFlag{
				Name:  "include-transitive-needs",
				Usage: `like --include-needs, but also includes transitive needs (needs of needs). Does nothing when when --selector/-l flag is not provided. Overrides exclusions of other selectors and conditions.`,
			},
			cli.BoolFlag{
				Name:  "validate",
				Usage: `ADVANCED CONFIGURATION: When sync is going to involve helm-template as a part of the "chartify" process, it might fail due to missing .Capabilities. This flag makes instructs helmfile to pass --validate to helm-template so it populates .Capabilities and validates your manifests against the Kubernetes cluster you are currently pointing at. Note that this requires access to a Kubernetes cluster to obtain information necessary for validating, like the list of available API versions`,
			},
			cli.BoolFlag{
				Name:  "wait",
				Usage: `Override helmDefaults.wait setting "helm upgrade --install --wait"`,
			},
			cli.BoolFlag{
				Name:  "wait-for-jobs",
				Usage: `Override helmDefaults.waitForJobs setting "helm upgrade --install --wait-for-jobs"`,
			},
		},
		Action: Action(func(a *app.App, c config.ConfigImpl) error {
			return a.Sync(c)
		}),
	})
}
