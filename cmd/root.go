package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/urfave/cli"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/app/version"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

var logger *zap.SugaredLogger
var globalUsage = "Declaratively deploy your Kubernetes manifests, Kustomize configs, and Charts as Helm releases in one shot"

func toCLIError(g *config.GlobalImpl, err error) error {
	if err != nil {
		switch e := err.(type) {
		case *app.NoMatchingHelmfileError:
			noMatchingExitCode := 3
			if g.AllowNoMatchingRelease {
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

// NewRootCmd creates the root command for the CLI.
func NewRootCmd(globalConfig *config.GlobalOptions, args []string) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:          "helmfile",
		Short:        globalUsage,
		Long:         globalUsage,
		Args:         cobra.MinimumNArgs(1),
		Version:      version.GetVersion(),
		SilenceUsage: true,
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			// Valid levels:
			// https://github.com/uber-go/zap/blob/7e7e266a8dbce911a49554b945538c5b950196b8/zapcore/level.go#L126
			logLevel := globalConfig.LogLevel
			switch {
			case globalConfig.Debug:
				logLevel = "debug"
			case globalConfig.Quiet:
				logLevel = "warn"
			}
			logger = helmexec.NewLogger(os.Stderr, logLevel)
			globalConfig.SetLogger(logger)
			return nil
		},
	}
	flags := cmd.PersistentFlags()

	// Set the global options for the root command.
	setGlobalOptionsForRootCmd(flags, globalConfig)

	// We can safely ignore any errors that flags.Parse encounters since
	// those errors will be caught later during the call to cmd.Execution.
	// This call is required to gather configuration information prior to
	// execution.
	flags.ParseErrorsWhitelist.UnknownFlags = true

	err := flags.Parse(args)
	if err != nil {
		return nil, err
	}
	globalImpl := config.NewGlobalImpl(globalConfig)
	cmd.AddCommand(
		NewApplyCmd(globalImpl),
		NewBuildCmd(globalImpl),
		NewCacheCmd(globalImpl),
		NewChartsCmd(globalImpl),
		NewDeleteCmd(globalImpl),
		NewDepsCmd(globalImpl),
		NewDestroyCmd(globalImpl),
		NewFetchCmd(globalImpl),
		NewListCmd(globalImpl),
		NewReposCmd(globalImpl),
		NewVersionCmd(),
		NewLintCmd(globalImpl),
		NewWriteValuesCmd(globalImpl),
		NewTestCmd(globalImpl),
		NewTemplateCmd(globalImpl),
		NewSyncCmd(globalImpl),
		NewDiffCmd(globalImpl),
		NewStatusCmd(globalImpl),
	)

	return cmd, nil
}

func setGlobalOptionsForRootCmd(fs *pflag.FlagSet, globalOptions *config.GlobalOptions) {
	fs.StringVarP(&globalOptions.HelmBinary, "helm-binary", "b", app.DefaultHelmBinary, "Path to the helm binary")
	fs.StringVarP(&globalOptions.File, "file", "f", "", "load config from file or directory. defaults to `helmfile.yaml` or `helmfile.d`(means `helmfile.d/*.yaml`) in this preference")
	fs.StringVarP(&globalOptions.Environment, "environment", "e", "", `specify the environment name. defaults to "default"`)
	fs.StringArrayVar(&globalOptions.StateValuesSet, "state-values-set", nil, "set state values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	fs.StringArrayVar(&globalOptions.StateValuesFile, "state-values-file", nil, "specify state values in a YAML file")
	fs.BoolVarP(&globalOptions.Quiet, "quiet", "q", false, "Silence output. Equivalent to log-level warn")
	fs.StringVar(&globalOptions.KubeContext, "kube-context", "", "Set kubectl context. Uses current context by default")
	fs.BoolVar(&globalOptions.Debug, "debug", false, "Enable verbose output for Helm and set log-level to debug, this disables --quiet/-q effect")
	fs.BoolVar(&globalOptions.Color, "color", false, "Output with color")
	fs.BoolVar(&globalOptions.NoColor, "no-color", false, "Output without color")
	fs.StringVar(&globalOptions.LogLevel, "log-level", "info", "Set log level, default info")
	fs.StringVarP(&globalOptions.Namespace, "namespace", "n", "", "Set namespace. Uses the namespace set in the context by default, and is available in templates as {{ .Namespace }}")
	fs.StringVarP(&globalOptions.Chart, "chart", "c", "", "Set chart. Uses the chart set in release by default, and is available in template as {{ .Chart }}")
	fs.StringArrayVarP(&globalOptions.Selector, "selector", "l", nil, `Only run using the releases that match labels. Labels can take the form of foo=bar or foo!=bar.
	A release must match all labels in a group in order to be used. Multiple groups can be specified at once.
	--selector tier=frontend,tier!=proxy --selector tier=backend. Will match all frontend, non-proxy releases AND all backend releases.
	The name of a release can be used as a label. --selector name=myrelease`)
	fs.BoolVar(&globalOptions.AllowNoMatchingRelease, "allow-no-matching-release", false, `Do not exit with an error code if the provided selector has no matching releases.`)
	// avoid 'pflag: help requested' error (#251)
	fs.BoolP("help", "h", false, "help for helmfile")
}
