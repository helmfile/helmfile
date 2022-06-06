package cmd

import (
	"fmt"
	"os"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/app/version"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/urfave/cli"
	"go.uber.org/zap"
)

var logger *zap.SugaredLogger

// RootCmd is the root command for helmfile.
func RootCommand() *cli.App {
	cliApp := cli.NewApp()
	cliApp.Name = "helmfile"
	cliApp.Usage = ""
	cliApp.Version = version.Version
	cliApp.EnableBashCompletion = true
	cliApp.Before = configureLogging
	setRootCommandFlags(cliApp)

	// add subcommands
	addDepsSubcommand(cliApp)
	addReposSubcommand(cliApp)
	addChartsSubcommand(cliApp)
	addDiffSubcommand(cliApp)
	addTemplateSubcommand(cliApp)

	return cliApp
}

// setRootCommandFlags sets the flags for the root command.
func setRootCommandFlags(cliApp *cli.App) {
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

func Action(do func(*app.App, config.ConfigImpl) error) func(*cli.Context) error {
	return func(implCtx *cli.Context) error {
		conf, err := config.NewUrfaveCliConfigImpl(implCtx)
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
