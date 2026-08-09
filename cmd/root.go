package cmd

import (
	stderrors "errors"
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
		var exitErr helmexec.ExitError
		if stderrors.As(err, &exitErr) {
			return errors.NewExitError(exitErr.Error(), exitErr.ExitStatus())
		}
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
			return errors.NewExitError(fmt.Sprintf("unexpected error: %T: %v", e, e), 1)
		}
	}
	return err
}

// NewRootCmd creates the root command for the CLI.
func NewRootCmd(globalConfig *config.GlobalOptions) (*cobra.Command, error) {
	globalImpl := config.NewGlobalImpl(globalConfig)

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
			logLevel := globalImpl.LogLevel()
			switch {
			case globalImpl.Debug():
				logLevel = "debug"
			case globalImpl.Quiet():
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

	flags.ParseErrorsAllowlist.UnknownFlags = true

	// when set environment HELMFILE_UPGRADE_NOTICE_DISABLED any value, skip upgrade notice.
	var versionOpts []extension.CobraOption
	if os.Getenv(envvar.UpgradeNoticeDisabled) == "" {
		versionOpts = append(versionOpts, extension.WithUpgradeNotice("helmfile", "helmfile"))
	}

	cmd.AddCommand(
		NewCreateCmd(globalImpl),
		NewInitCmd(globalImpl),
		NewApplyCmd(globalImpl),
		NewBuildCmd(globalImpl),
		NewCacheCmd(globalImpl),
		NewDepsCmd(globalImpl),
		NewDestroyCmd(globalImpl),
		NewDiffCmd(globalImpl),
		NewDoctorCmd(globalImpl),
		NewFetchCmd(globalImpl),
		NewListCmd(globalImpl),
		NewReposCmd(globalImpl),
		NewLintCmd(globalImpl),
		NewWriteValuesCmd(globalImpl),
		NewTestCmd(globalImpl),
		NewUnittestCmd(globalImpl),
		NewTemplateCmd(globalImpl),
		NewSyncCmd(globalImpl),
		NewStatusCmd(globalImpl),
		NewShowDAGCmd(globalImpl),
		NewPrintEnvCmd(globalImpl),
		extension.NewVersionCobraCmd(
			versionOpts...,
		),
	)

	return cmd, nil
}

func setGlobalOptionsForRootCmd(fs *pflag.FlagSet, globalOptions *config.GlobalOptions) {
	fs.StringVarP(&globalOptions.HelmBinary, "helm-binary", "b", "", fmt.Sprintf(`Path to the helm binary. Overrides "HELMFILE_HELM_BINARY" OS environment variable when specified (default %q)`, app.DefaultHelmBinary))
	fs.StringVarP(&globalOptions.KustomizeBinary, "kustomize-binary", "k", "", fmt.Sprintf(`Path to the kustomize binary. Overrides "HELMFILE_KUSTOMIZE_BINARY" OS environment variable when specified (default %q)`, app.DefaultKustomizeBinary))
	fs.StringVarP(&globalOptions.File, "file", "f", "", "load config from file or directory. defaults to \"`helmfile.yaml`\" or \"helmfile.yaml.gotmpl\" or \"helmfile.d\" (means \"helmfile.d/*.yaml\" or \"helmfile.d/*.yaml.gotmpl\") in this preference. Specify - to load the config from the standard input.")
	fs.StringVarP(&globalOptions.Environment, "environment", "e", "", `specify the environment name. Overrides "HELMFILE_ENVIRONMENT" OS environment variable when specified. defaults to "default"`)
	fs.StringArrayVar(&globalOptions.StateValuesSet, "state-values-set", nil, "set state values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2). Used to override .Values within the helmfile template (not values template).")
	fs.StringArrayVar(&globalOptions.StateValuesSetString, "state-values-set-string", nil, "set state STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2). Used to override .Values within the helmfile template (not values template).")
	fs.StringArrayVar(&globalOptions.StateValuesFile, "state-values-file", nil, "specify state values in a YAML file. Used to override .Values within the helmfile template (not values template).")
	fs.BoolVar(&globalOptions.SkipDeps, "skip-deps", false, `skip running "helm repo update" and "helm dependency build"`)
	fs.BoolVar(&globalOptions.SkipRefresh, "skip-refresh", false, `skip running "helm repo update"`)
	fs.BoolVar(&globalOptions.AllowFailedReleases, "allow-failed-releases", false, `continue preparing charts for other releases when chart preparation fails for a release; report failures at the end`)
	fs.BoolVar(&globalOptions.StripArgsValuesOnExitError, "strip-args-values-on-exit-error", true, `Strip the potential secret values of the helm command args contained in a helmfile error message`)
	fs.BoolVar(&globalOptions.DisableForceUpdate, "disable-force-update", false, `do not force helm repos to update when executing "helm repo add" (Helm 3 only)`)
	fs.BoolVar(&globalOptions.EnforcePluginVerification, "enforce-plugin-verification", false, `fail plugin installation if verification is not supported (for security purposes)`)
	fs.BoolVar(&globalOptions.HelmOCIPlainHTTP, "oci-plain-http", false, `use plain HTTP for OCI registries (required for local/insecure registries in Helm 4)`)
	fs.IntVar(&globalOptions.RepoRetry, "repo-retries", -1, `Number of times to retry "helm repo add/update" and "helm registry login" on failure, with exponential backoff (1s, 2s, 4s, ..., capped at 30s). Set to 0 to disable retries. Overrides "HELMFILE_REPO_RETRIES" OS environment variable when specified`)
	// The actual default is -1 (a sentinel meaning "flag not set, fall back to
	// the env var"); display "0" in --help to match the documented default and
	// avoid confusing users with a negative value.
	if f := fs.Lookup("repo-retries"); f != nil {
		f.DefValue = "0"
	}
	fs.BoolVarP(&globalOptions.Quiet, "quiet", "q", false, `Silence output. Equivalent to log-level warn. Overrides "HELMFILE_QUIET" OS environment variable when specified`)
	fs.StringVar(&globalOptions.Kubeconfig, "kubeconfig", "", "Use a particular kubeconfig file")
	fs.StringVar(&globalOptions.KubeContext, "kube-context", "", `Set kubectl context. Overrides "HELMFILE_KUBE_CONTEXT" OS environment variable when specified. Uses current kubectl context by default`)
	fs.BoolVar(&globalOptions.Debug, "debug", false, `Enable verbose output for Helm and set log-level to debug, this disables --quiet/-q effect. Overrides "HELMFILE_DEBUG" OS environment variable when specified`)
	fs.BoolVar(&globalOptions.Color, "color", false, "Output with color")
	fs.BoolVar(&globalOptions.NoColor, "no-color", false, `Output without color. Overrides "HELMFILE_NO_COLOR" and "NO_COLOR" OS environment variables when specified`)
	fs.StringVar(&globalOptions.LogLevel, "log-level", "", `Set log level. Overrides "HELMFILE_LOG_LEVEL" OS environment variable when specified (default "info")`)
	fs.StringVarP(&globalOptions.Namespace, "namespace", "n", "", `Set namespace. Overrides "HELMFILE_NAMESPACE" OS environment variable when specified. Uses the namespace set in the context by default, and is available in templates as {{ .Namespace }}`)
	fs.StringVarP(&globalOptions.Chart, "chart", "c", "", "Set chart. Uses the chart set in release by default, and is available in template as {{ .Chart }}")
	fs.StringArrayVarP(&globalOptions.Selector, "selector", "l", nil, `Only run using the releases that match labels. Labels can take the form of foo=bar or foo!=bar.
A release must match all labels in a group in order to be used. Multiple groups can be specified at once.
"--selector tier=frontend,tier!=proxy --selector tier=backend" will match all frontend, non-proxy releases AND all backend releases.
The name of a release can be used as a label: "--selector name=myrelease"`)
	fs.BoolVar(&globalOptions.AllowNoMatchingRelease, "allow-no-matching-release", false, `Do not exit with an error code if the provided selector has no matching releases.`)
	fs.BoolVar(&globalOptions.EnableLiveOutput, "enable-live-output", globalOptions.EnableLiveOutput, `Show live output from the Helm binary Stdout/Stderr into Helmfile own Stdout/Stderr.
It only applies for the Helm CLI commands, Stdout/Stderr for Hooks are still displayed only when it's execution finishes.`)
	fs.BoolVarP(&globalOptions.Interactive, "interactive", "i", false, "Request confirmation before attempting to modify clusters")
	fs.BoolVar(&globalOptions.SequentialHelmfiles, "sequential-helmfiles", false, `Process helmfile.d files sequentially in alphabetical order instead of in parallel.
Useful when file order matters for dependencies (e.g., databases before applications).
When processing multiple files, paths are resolved without changing the process working directory,
so relative environment variables like KUBECONFIG work correctly.`)
	// avoid 'pflag: help requested' error (#251)
	fs.BoolP("help", "h", false, "help for helmfile")
}
