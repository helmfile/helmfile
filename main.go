package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/app/version"
	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/maputil"
	"github.com/helmfile/helmfile/pkg/state"
	"github.com/urfave/cli"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"
)

var logger *zap.SugaredLogger

func configureLogging(c *cli.Context) error {
	// Valid levels:
	// https://github.com/uber-go/zap/blob/7e7e266a8dbce911a49554b945538c5b950196b8/zapcore/level.go#L126
	logLevel := c.GlobalString("log-level")
	if c.GlobalBool("debug") {
		logLevel = "debug"
	} else if c.GlobalBool("quiet") {
		logLevel = "warn"
	}
	logger = helmexec.NewLogger(os.Stderr, logLevel)
	if c.App.Metadata == nil {
		// Auto-initialised in 1.19.0
		// https://github.com/urfave/cli/blob/master/CHANGELOG.md#1190---2016-11-19
		c.App.Metadata = make(map[string]interface{})
	}
	c.App.Metadata["logger"] = logger
	return nil
}

func main() {

	cliApp := cli.NewApp()
	cliApp.Name = "helmfile"
	cliApp.Usage = ""
	cliApp.Version = version.Version
	cliApp.EnableBashCompletion = true
	cliApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "helm-binary, b",
			Usage: "path to helm binary",
			Value: app.DefaultHelmBinary,
		},
		cli.StringFlag{
			Name:  "file, f",
			Usage: "load config from file or directory. defaults to `helmfile.yaml` or `helmfile.d`(means `helmfile.d/*.yaml`) in this preference",
		},
		cli.StringFlag{
			Name:  "environment, e",
			Usage: `specify the environment name. defaults to "default"`,
		},
		cli.StringSliceFlag{
			Name:  "state-values-set",
			Usage: "set state values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)",
		},
		cli.StringSliceFlag{
			Name:  "state-values-file",
			Usage: "specify state values in a YAML file",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Silence output. Equivalent to log-level warn",
		},
		cli.StringFlag{
			Name:  "kube-context",
			Usage: "Set kubectl context. Uses current context by default",
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Enable verbose output for Helm and set log-level to debug, this disables --quiet/-q effect",
		},
		cli.BoolFlag{
			Name:  "color",
			Usage: "Output with color",
		},
		cli.BoolFlag{
			Name:  "no-color",
			Usage: "Output without color",
		},
		cli.StringFlag{
			Name:  "log-level",
			Usage: "Set log level, default info",
		},
		cli.StringFlag{
			Name:  "namespace, n",
			Usage: "Set namespace. Uses the namespace set in the context by default, and is available in templates as {{ .Namespace }}",
		},
		cli.StringFlag{
			Name:  "chart, c",
			Usage: "Set chart. Uses the chart set in release by default, and is available in template as {{ .Chart }}",
		},
		cli.StringSliceFlag{
			Name: "selector, l",
			Usage: `Only run using the releases that match labels. Labels can take the form of foo=bar or foo!=bar.
	A release must match all labels in a group in order to be used. Multiple groups can be specified at once.
	--selector tier=frontend,tier!=proxy --selector tier=backend. Will match all frontend, non-proxy releases AND all backend releases.
	The name of a release can be used as a label. --selector name=myrelease`,
		},
		cli.BoolFlag{
			Name:  "allow-no-matching-release",
			Usage: `Do not exit with an error code if the provided selector has no matching releases.`,
		},
		cli.BoolFlag{
			Name:  "interactive, i",
			Usage: "Request confirmation before attempting to modify clusters",
		},
	}

	cliApp.Before = configureLogging
	cliApp.Commands = []cli.Command{
		{
			Name:  "deps",
			Usage: "update charts based on their requirements",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "args",
					Value: "",
					Usage: "pass args to helm exec",
				},
				cli.BoolFlag{
					Name:  "skip-repos",
					Usage: `skip running "helm repo update" before running "helm dependency build"`,
				},
			},
			Action: action(func(a *app.App, c configImpl) error {
				return a.Deps(c)
			}),
		},
		{
			Name:  "repos",
			Usage: "sync repositories from state file (helm repo add && helm repo update)",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "args",
					Value: "",
					Usage: "pass args to helm exec",
				},
			},
			Action: action(func(a *app.App, c configImpl) error {
				return a.Repos(c)
			}),
		},
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
			Action: action(func(a *app.App, c configImpl) error {
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
			Action: action(func(a *app.App, c configImpl) error {
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
			Action: action(func(a *app.App, c configImpl) error {
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
			Action: action(func(a *app.App, c configImpl) error {
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
			Action: action(func(a *app.App, c configImpl) error {
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
			Action: action(func(a *app.App, c configImpl) error {
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
			Action: action(func(a *app.App, c configImpl) error {
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
			Action: action(func(a *app.App, c configImpl) error {
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
			Action: action(func(a *app.App, c configImpl) error {
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
			Action: action(func(a *app.App, c configImpl) error {
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
			Action: action(func(a *app.App, c configImpl) error {
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
			Action: action(func(a *app.App, c configImpl) error {
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
			Action: action(func(a *app.App, c configImpl) error {
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
			Action: action(func(a *app.App, c configImpl) error {
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
					Action: action(func(a *app.App, c configImpl) error {
						return a.ShowCacheDir(c)
					}),
				},
				{
					Name:  "cleanup",
					Usage: "clean up cache directory",
					Action: action(func(a *app.App, c configImpl) error {
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

	err := cliApp.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(3)
	}
}

type configImpl struct {
	c *cli.Context

	set map[string]interface{}
}

func NewUrfaveCliConfigImpl(c *cli.Context) (configImpl, error) {
	if c.NArg() > 0 {
		err := cli.ShowAppHelp(c)
		if err != nil {
			return configImpl{}, err
		}
		return configImpl{}, fmt.Errorf("err: extraneous arguments: %s", strings.Join(c.Args(), ", "))
	}

	conf := configImpl{
		c: c,
	}

	optsSet := c.GlobalStringSlice("state-values-set")
	if len(optsSet) > 0 {
		set := map[string]interface{}{}
		for i := range optsSet {
			ops := strings.Split(optsSet[i], ",")
			for j := range ops {
				op := strings.SplitN(ops[j], "=", 2)
				k := maputil.ParseKey(op[0])
				v := op[1]

				maputil.Set(set, k, v)
			}
		}
		conf.set = set
	}

	return conf, nil
}

func (c configImpl) Set() []string {
	return c.c.StringSlice("set")
}

func (c configImpl) SkipRepos() bool {
	return c.c.Bool("skip-repos")
}

func (c configImpl) Wait() bool {
	return c.c.Bool("wait")
}

func (c configImpl) WaitForJobs() bool {
	return c.c.Bool("wait-for-jobs")
}

func (c configImpl) Values() []string {
	return c.c.StringSlice("values")
}

func (c configImpl) Args() string {
	args := c.c.String("args")
	enableHelmDebug := c.c.GlobalBool("debug")

	if enableHelmDebug {
		args = fmt.Sprintf("%s %s", args, "--debug")
	}
	return args
}

func (c configImpl) OutputDir() string {
	return strings.TrimRight(c.c.String("output-dir"), fmt.Sprintf("%c", os.PathSeparator))
}

func (c configImpl) OutputDirTemplate() string {
	return c.c.String("output-dir-template")
}

func (c configImpl) OutputFileTemplate() string {
	return c.c.String("output-file-template")
}

func (c configImpl) Validate() bool {
	return c.c.Bool("validate")
}

func (c configImpl) Concurrency() int {
	return c.c.Int("concurrency")
}

func (c configImpl) HasCommandName(name string) bool {
	return c.c.Command.HasName(name)
}

func (c configImpl) SkipNeeds() bool {
	if !c.IncludeNeeds() {
		return c.c.Bool("skip-needs")
	}

	return false
}

func (c configImpl) IncludeNeeds() bool {
	return c.c.Bool("include-needs") || c.IncludeTransitiveNeeds()
}

func (c configImpl) IncludeTransitiveNeeds() bool {
	return c.c.Bool("include-transitive-needs")
}

// DiffConfig

func (c configImpl) SkipDeps() bool {
	return c.c.Bool("skip-deps")
}

func (c configImpl) DetailedExitcode() bool {
	return c.c.Bool("detailed-exitcode")
}

func (c configImpl) RetainValuesFiles() bool {
	return c.c.Bool("retain-values-files")
}

func (c configImpl) IncludeTests() bool {
	return c.c.Bool("include-tests")
}

func (c configImpl) Suppress() []string {
	return c.c.StringSlice("suppress")
}

func (c configImpl) SuppressSecrets() bool {
	return c.c.Bool("suppress-secrets")
}

func (c configImpl) ShowSecrets() bool {
	return c.c.Bool("show-secrets")
}

func (c configImpl) SuppressDiff() bool {
	return c.c.Bool("suppress-diff")
}

// DeleteConfig

func (c configImpl) Purge() bool {
	return c.c.Bool("purge")
}

// TestConfig

func (c configImpl) Cleanup() bool {
	return c.c.Bool("cleanup")
}

func (c configImpl) Logs() bool {
	return c.c.Bool("logs")
}

func (c configImpl) Timeout() int {
	if !c.c.IsSet("timeout") {
		return state.EmptyTimeout
	}
	return c.c.Int("timeout")
}

// ListConfig

func (c configImpl) Output() string {
	return c.c.String("output")
}

func (c configImpl) KeepTempDir() bool {
	return c.c.Bool("keep-temp-dir")
}

// GlobalConfig

func (c configImpl) HelmBinary() string {
	return c.c.GlobalString("helm-binary")
}

func (c configImpl) KubeContext() string {
	return c.c.GlobalString("kube-context")
}

func (c configImpl) Namespace() string {
	return c.c.GlobalString("namespace")
}

func (c configImpl) Chart() string {
	return c.c.GlobalString("chart")
}

func (c configImpl) FileOrDir() string {
	return c.c.GlobalString("file")
}

func (c configImpl) Selectors() []string {
	return c.c.GlobalStringSlice("selector")
}

func (c configImpl) StateValuesSet() map[string]interface{} {
	return c.set
}

func (c configImpl) StateValuesFiles() []string {
	return c.c.GlobalStringSlice("state-values-file")
}

func (c configImpl) Interactive() bool {
	return c.c.GlobalBool("interactive")
}

func (c configImpl) Color() bool {
	if c := c.c.GlobalBool("color"); c {
		return c
	}

	if c.NoColor() {
		return false
	}

	// We replicate the helm-diff behavior in helmfile
	// because when when helmfile calls helm-diff, helm-diff has no access to term and therefore
	// we can't rely on helm-diff's ability to auto-detect term for color output.
	// See https://github.com/roboll/helmfile/issues/2043

	term := terminal.IsTerminal(int(os.Stdout.Fd()))
	// https://github.com/databus23/helm-diff/issues/281
	dumb := os.Getenv("TERM") == "dumb"
	return term && !dumb
}

func (c configImpl) NoColor() bool {
	return c.c.GlobalBool("no-color")
}

func (c configImpl) Context() int {
	return c.c.Int("context")
}

func (c configImpl) DiffOutput() string {
	return c.c.String("output")
}

func (c configImpl) SkipCleanup() bool {
	return c.c.Bool("skip-cleanup")
}

func (c configImpl) SkipCRDs() bool {
	return c.c.Bool("skip-crds")
}

func (c configImpl) SkipDiffOnInstall() bool {
	return c.c.Bool("skip-diff-on-install")
}

func (c configImpl) EmbedValues() bool {
	return c.c.Bool("embed-values")
}

func (c configImpl) IncludeCRDs() bool {
	return c.c.Bool("include-crds")
}

func (c configImpl) SkipTests() bool {
	return c.c.Bool("skip-tests")
}

func (c configImpl) Logger() *zap.SugaredLogger {
	return c.c.App.Metadata["logger"].(*zap.SugaredLogger)
}

func (c configImpl) Env() string {
	env := c.c.GlobalString("environment")
	if env == "" {
		env = os.Getenv(envvar.Environment)
		if env == "" {
			env = state.DefaultEnv
		}
	}
	return env
}

func action(do func(*app.App, configImpl) error) func(*cli.Context) error {
	return func(implCtx *cli.Context) error {
		conf, err := NewUrfaveCliConfigImpl(implCtx)
		if err != nil {
			return err
		}

		if err := app.ValidateConfig(conf); err != nil {
			return err
		}

		a := app.New(conf)

		if err := do(a, conf); err != nil {
			return toCliError(implCtx, err)
		}

		return nil
	}
}

func toCliError(c *cli.Context, err error) error {
	if err != nil {
		switch e := err.(type) {
		case *app.NoMatchingHelmfileError:
			noMatchingExitCode := 3
			if c.GlobalBool("allow-no-matching-release") {
				noMatchingExitCode = 0
			}
			return cli.NewExitError(e.Error(), noMatchingExitCode)
		case *app.MultiError:
			return cli.NewExitError(e.Error(), 1)
		case *app.Error:
			return cli.NewExitError(e.Error(), e.Code())
		default:
			panic(fmt.Errorf("BUG: please file an github issue for this unhandled error: %T: %v", e, e))
		}
	}
	return err
}
