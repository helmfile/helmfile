package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.szostok.io/version/extension"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/app/version"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/errors"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/runtime"
)

var logger *zap.SugaredLogger
var globalUsage = "Declaratively deploy your Kubernetes manifests, Kustomize configs, and Charts as Helm releases in one shot\n" + runtime.Info()

func toCLIError(g *config.GlobalImpl, err error) error {
	if err != nil {
		switch e := err.(type) {
		case *app.NoMatchingHelmfileError:
			noMatchingExitCode := 3
			if g.AllowNoMatchingRelease {
				noMatchingExitCode = 0
			}
			return errors.NewExitError(e.Error(), noMatchingExitCode)
		case *app.MultiError:
			return errors.NewExitError(e.Error(), 1)
		case *app.Error:
			return errors.NewExitError(e.Error(), e.Code())
		default:
			panic(fmt.Errorf("BUG: please file an github issue for this unhandled error: %T: %v", e, e))
		}
	}
	return err
}

// NewRootCmd creates the root command for the CLI.
func NewRootCmd(globalConfig *config.GlobalOptions) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:           "helmfile",
		Short:         globalUsage,
		Long:          globalUsage,
		Version:       version.Version(),
		SilenceUsage:  true,
		SilenceErrors: true,
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

			// If the log output is not set, default to stderr.
			logOut := globalConfig.LogOutput
			if logOut == nil {
				logOut = os.Stderr
			}
			logger = helmexec.NewLogger(logOut, logLevel)
			globalConfig.SetLogger(logger)
			return nil
		},
	}
	flags := cmd.PersistentFlags()

	// Set the global options for the root command.
	setGlobalOptionsForRootCmd(flags, globalConfig)

	flags.ParseErrorsWhitelist.UnknownFlags = true

	globalImpl := config.NewGlobalImpl(globalConfig)

	// when set environment HELMFILE_UPGRADE_NOTICE_DISABLED any value, skip upgrade notice.
	var versionOpts []extension.CobraOption
	if os.Getenv(envvar.UpgradeNoticeDisabled) == "" {
		versionOpts = append(versionOpts, extension.WithUpgradeNotice("helmfile", "helmfile"))
	}

	cmd.AddCommand(
		NewInitCmd(globalImpl),
		NewApplyCmd(globalImpl),
		NewBuildCmd(globalImpl),
		NewCacheCmd(globalImpl),
		NewDepsCmd(globalImpl),
		NewDestroyCmd(globalImpl),
		NewFetchCmd(globalImpl),
		NewListCmd(globalImpl),
		NewReposCmd(globalImpl),
		NewLintCmd(globalImpl),
		NewWriteValuesCmd(globalImpl),
		NewTestCmd(globalImpl),
		NewTemplateCmd(globalImpl),
		NewSyncCmd(globalImpl),
		NewDiffCmd(globalImpl),
		NewStatusCmd(globalImpl),
		NewShowDAGCmd(globalImpl),
		extension.NewVersionCobraCmd(
			versionOpts...,
		),
	)

	return cmd, nil
}

func setGlobalOptionsForRootCmd(fs *pflag.FlagSet, globalOptions *config.GlobalOptions) {
	fs.StringVarP(&globalOptions.HelmBinary, "helm-binary", "b", app.DefaultHelmBinary, "Path to the helm binary")
	fs.StringVarP(&globalOptions.KustomizeBinary, "kustomize-binary", "k", app.DefaultKustomizeBinary, "Path to the kustomize binary")
	fs.StringVarP(&globalOptions.File, "file", "f", "", "load config from file or directory. defaults to \"`helmfile.yaml`\" or \"helmfile.yaml.gotmpl\" or \"helmfile.d\" (means \"helmfile.d/*.yaml\" or \"helmfile.d/*.yaml.gotmpl\") in this preference. Specify - to load the config from the standard input.")
	fs.StringVarP(&globalOptions.Environment, "environment", "e", "", `specify the environment name. Overrides "HELMFILE_ENVIRONMENT" OS environment variable when specified. defaults to "default"`)
	fs.StringArrayVar(&globalOptions.StateValuesSet, "state-values-set", nil, "set state values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2). Used to override .Values within the helmfile template (not values template).")
	fs.StringArrayVar(&globalOptions.StateValuesSetString, "state-values-set-string", nil, "set state STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2). Used to override .Values within the helmfile template (not values template).")
	fs.StringArrayVar(&globalOptions.StateValuesFile, "state-values-file", nil, "specify state values in a YAML file. Used to override .Values within the helmfile template (not values template).")
	fs.BoolVar(&globalOptions.SkipDeps, "skip-deps", false, `skip running "helm repo update" and "helm dependency build"`)
	fs.BoolVar(&globalOptions.SkipRefresh, "skip-refresh", false, `skip running "helm repo update"`)
	fs.BoolVar(&globalOptions.StripArgsValuesOnExitError, "strip-args-values-on-exit-error", true, `Strip the potential secret values of the helm command args contained in a helmfile error message`)
	fs.BoolVar(&globalOptions.DisableForceUpdate, "disable-force-update", false, `do not force helm repos to update when executing "helm repo add"`)
	fs.BoolVarP(&globalOptions.Quiet, "quiet", "q", false, "Silence output. Equivalent to log-level warn")
	fs.StringVar(&globalOptions.Kubeconfig, "kubeconfig", "", "Use a particular kubeconfig file")
	fs.StringVar(&globalOptions.KubeContext, "kube-context", "", "Set kubectl context. Uses current context by default")
	fs.BoolVar(&globalOptions.Debug, "debug", false, "Enable verbose output for Helm and set log-level to debug, this disables --quiet/-q effect")
	fs.BoolVar(&globalOptions.Color, "color", false, "Output with color")
	fs.BoolVar(&globalOptions.NoColor, "no-color", false, "Output without color")
	fs.StringVar(&globalOptions.LogLevel, "log-level", "info", "Set log level, default info")
	fs.StringVarP(&globalOptions.Namespace, "namespace", "n", "", "Set namespace. Uses the namespace set in the context by default, and is available in templates as {{ .Namespace }}")
	fs.StringVarP(&globalOptions.Chart, "chart", "c", "", "Set chart. Uses the chart set in release by default, and is available in template as {{ .Chart }}")
	fs.StringArrayVarP(&globalOptions.Selector, "selector", "l", nil, `Only run using the releases that match labels. Labels can take the form of foo=bar or foo!=bar.
A release must match all labels in a group in order to be used. Multiple groups can be specified at once.
"--selector tier=frontend,tier!=proxy --selector tier=backend" will match all frontend, non-proxy releases AND all backend releases.
The name of a release can be used as a label: "--selector name=myrelease"`)
	fs.BoolVar(&globalOptions.AllowNoMatchingRelease, "allow-no-matching-release", false, `Do not exit with an error code if the provided selector has no matching releases.`)
	fs.BoolVar(&globalOptions.EnableLiveOutput, "enable-live-output", globalOptions.EnableLiveOutput, `Show live output from the Helm binary Stdout/Stderr into Helmfile own Stdout/Stderr.
It only applies for the Helm CLI commands, Stdout/Stderr for Hooks are still displayed only when it's execution finishes.`)
	fs.BoolVarP(&globalOptions.Interactive, "interactive", "i", false, "Request confirmation before attempting to modify clusters")
	// avoid 'pflag: help requested' error (#251)
	fs.BoolP("help", "h", false, "help for helmfile")
}
