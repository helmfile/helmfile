package main

import (
	"fmt"
	"os"

	"github.com/helmfile/helmfile/cmd"
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/urfave/cli"
)

func main() {

	rootCmd := cmd.RootCommand()
	subCommands := []cli.Command{
		{
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
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.DeprecatedSyncCharts(c)
			}),
		},
		{
			Name:  "diff",
			Usage: "diff releases from state file against env (helm diff)",
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
				cli.BoolFlag{
					Name:  "skip-deps",
					Usage: `skip running "helm repo update" and "helm dependency build"`,
				},
				cli.BoolFlag{
					Name:  "detailed-exitcode",
					Usage: "return a non-zero exit code when there are changes",
				},
				cli.BoolFlag{
					Name:  "include-tests",
					Usage: "enable the diffing of the helm test hooks",
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
					Name:  "skip-diff-on-install",
					Usage: "Skips running helm-diff on releases being newly installed on this apply. Useful when the release manifests are too huge to be reviewed, or it's too time-consuming to diff at all",
				},
				cli.StringSliceFlag{
					Name:  "suppress",
					Usage: "suppress specified Kubernetes objects in the output. Can be provided multiple times. For example: --suppress KeycloakClient --suppress VaultSecret",
				},
				cli.BoolFlag{
					Name:  "suppress-secrets",
					Usage: "suppress secrets in the output. highly recommended to specify on CI/CD use-cases",
				},
				cli.BoolFlag{
					Name:  "show-secrets",
					Usage: "do not redact secret values in the output. should be used for debug purpose only",
				},
				cli.IntFlag{
					Name:  "concurrency",
					Value: 0,
					Usage: "maximum number of concurrent helm processes to run, 0 is unlimited",
				},
				cli.BoolFlag{
					Name:  "validate",
					Usage: "validate your manifests against the Kubernetes cluster you are currently pointing at. Note that this requiers access to a Kubernetes cluster to obtain information necessary for validating, like the list of available API versions",
				},

				cli.IntFlag{
					Name:  "context",
					Value: 0,
					Usage: "output NUM lines of context around changes",
				},
				cli.StringFlag{
					Name:  "output",
					Value: "",
					Usage: "output format for diff plugin",
				},
			},
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.Diff(c)
			}),
		},
		{
			Name:  "template",
			Usage: "template releases from state file against env (helm template)",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "args",
					Value: "",
					Usage: "pass args to helm template",
				},
				cli.StringSliceFlag{
					Name:  "set",
					Usage: "additional values to be merged into the command",
				},
				cli.StringSliceFlag{
					Name:  "values",
					Usage: "additional value files to be merged into the command",
				},
				cli.StringFlag{
					Name:  "output-dir",
					Usage: "output directory to pass to helm template (helm template --output-dir)",
				},
				cli.StringFlag{
					Name:  "output-dir-template",
					Usage: "go text template for generating the output directory. Default: {{ .OutputDir }}/{{ .State.BaseName }}-{{ .State.AbsPathSHA1 }}-{{ .Release.Name}}",
				},
				cli.IntFlag{
					Name:  "concurrency",
					Value: 0,
					Usage: "maximum number of concurrent downloads of release charts",
				},
				cli.BoolFlag{
					Name:  "validate",
					Usage: "validate your manifests against the Kubernetes cluster you are currently pointing at. Note that this requiers access to a Kubernetes cluster to obtain information necessary for validating, like the list of available API versions",
				},
				cli.BoolFlag{
					Name:  "include-crds",
					Usage: "include CRDs in the templated output",
				},
				cli.BoolFlag{
					Name:  "skip-tests",
					Usage: "skip tests from templated output",
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
					Name:  "skip-deps",
					Usage: `skip running "helm repo update" and "helm dependency build"`,
				},
				cli.BoolFlag{
					Name:  "skip-cleanup",
					Usage: "Stop cleaning up temporary values generated by helmfile and helm-secrets. Useful for debugging. Don't use in production for security",
				},
			},
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.Template(c)
			}),
		},
		{
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
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.WriteValues(c)
			}),
		},
		{
			Name:  "lint",
			Usage: "lint charts from state file (helm lint)",
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
					Usage: "maximum number of concurrent downloads of release charts",
				},
				cli.BoolFlag{
					Name:  "skip-deps",
					Usage: `skip running "helm repo update" and "helm dependency build"`,
				},
			},
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.Lint(c)
			}),
		},
		{
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
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.Fetch(c)
			}),
		},
		{
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
					Usage: `ADVANCED CONFIGURATION: When sync is going to involve helm-template as a part of the "chartify" process, it might fail due to missing .Capabilities. This flag makes instructs helmfile to pass --validate to helm-template so it populates .Capabilities and validates your manifests against the Kubernetes cluster you are currently pointing at. Note that this requiers access to a Kubernetes cluster to obtain information necessary for validating, like the list of available API versions`,
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
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.Sync(c)
			}),
		},
		{
			Name:  "apply",
			Usage: "apply all resources from state file only when there are changes",
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
				cli.BoolFlag{
					Name:  "validate",
					Usage: "validate your manifests against the Kubernetes cluster you are currently pointing at. Note that this requiers access to a Kubernetes cluster to obtain information necessary for validating, like the list of available API versions",
				},
				cli.IntFlag{
					Name:  "context",
					Value: 0,
					Usage: "output NUM lines of context around changes",
				},
				cli.StringFlag{
					Name:  "output",
					Value: "",
					Usage: "output format for diff plugin",
				},
				cli.BoolFlag{
					Name:  "detailed-exitcode",
					Usage: "return a non-zero exit code 2 instead of 0 when there were changes detected AND the changes are synced successfully",
				},
				cli.StringFlag{
					Name:  "args",
					Value: "",
					Usage: "pass args to helm exec",
				},
				cli.BoolFlag{
					Name:  "retain-values-files",
					Usage: "DEPRECATED: Use skip-cleanup instead",
				},
				cli.BoolFlag{
					Name:  "skip-cleanup",
					Usage: "Stop cleaning up temporary values generated by helmfile and helm-secrets. Useful for debugging. Don't use in production for security",
				},
				cli.BoolFlag{
					Name:  "skip-crds",
					Usage: "if set, no CRDs will be installed on sync. By default, CRDs are installed if not already present",
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
					Name:  "skip-diff-on-install",
					Usage: "Skips running helm-diff on releases being newly installed on this apply. Useful when the release manifests are too huge to be reviewed, or it's too time-consuming to diff at all",
				},
				cli.BoolFlag{
					Name:  "include-tests",
					Usage: "enable the diffing of the helm test hooks",
				},
				cli.StringSliceFlag{
					Name:  "suppress",
					Usage: "suppress specified Kubernetes objects in the diff output. Can be provided multiple times. For example: --suppress KeycloakClient --suppress VaultSecret",
				},
				cli.BoolFlag{
					Name:  "suppress-secrets",
					Usage: "suppress secrets in the diff output. highly recommended to specify on CI/CD use-cases",
				},
				cli.BoolFlag{
					Name:  "show-secrets",
					Usage: "do not redact secret values in the diff output. should be used for debug purpose only",
				},
				cli.BoolFlag{
					Name:  "suppress-diff",
					Usage: "suppress diff in the output. Usable in new installs",
				},
				cli.BoolFlag{
					Name:  "skip-deps",
					Usage: `skip running "helm repo update" and "helm dependency build"`,
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
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.Apply(c)
			}),
		},
		{
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
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.Status(c)
			}),
		},
		{
			Name:  "delete",
			Usage: "DEPRECATED: delete releases from state file (helm delete)",
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
					Name:  "purge",
					Usage: "purge releases i.e. free release names and histories",
				},
				cli.BoolFlag{
					Name:  "skip-deps",
					Usage: `skip running "helm repo update" and "helm dependency build"`,
				},
			},
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.Delete(c)
			}),
		},
		{
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
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.Destroy(c)
			}),
		},
		{
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
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.Test(c)
			}),
		},
		{
			Name:  "build",
			Usage: "output compiled helmfile state(s) as YAML",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "embed-values",
					Usage: "Read all the values files for every release and embed into the output helmfile.yaml",
				},
			},
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.PrintState(c)
			}),
		},
		{
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
			Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
				return a.ListReleases(c)
			}),
		},
		{
			Name:      "cache",
			Usage:     "cache management",
			ArgsUsage: "[command]",
			Subcommands: []cli.Command{
				{
					Name:  "info",
					Usage: "cache info",
					Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
						return a.ShowCacheDir(c)
					}),
				},
				{
					Name:  "cleanup",
					Usage: "clean up cache directory",
					Action: cmd.Action(func(a *app.App, c config.ConfigImpl) error {
						return a.CleanCacheDir(c)
					}),
				},
			},
		},
		{
			Name:      "version",
			Usage:     "Show the version for Helmfile.",
			ArgsUsage: "[command]",
			Action: func(c *cli.Context) error {
				cli.ShowVersion(c)
				return nil
			},
		},
	}
	rootCmd.Commands = append(rootCmd.Commands, subCommands...)

	err := rootCmd.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(3)
	}
}
