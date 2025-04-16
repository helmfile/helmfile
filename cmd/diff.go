package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/factory"
)

// NewDiffCmd returns diff subcmd
func NewDiffCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	optionsFactory := factory.NewDiffOptionsFactory()
	options := optionsFactory.CreateOptions().(*config.DiffOptions)
	flagRegistry := optionsFactory.GetFlagRegistry()

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff releases defined in state file",
		PreRun: func(cmd *cobra.Command, args []string) {
			flagRegistry.TransferFlags(cmd, options)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			diffImpl := config.NewDiffImpl(globalCfg, options)
			err := config.NewCLIConfigImpl(diffImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := diffImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(diffImpl)
			return toCLIError(diffImpl.GlobalImpl, a.Diff(diffImpl))
		},
	}

	f := cmd.Flags()
	f.StringVar(&options.DiffArgs, "diff-args", "", `pass args to helm helm-diff`)
	f.StringVar(&globalCfg.GlobalOptions.Args, "args", "", "pass args to helm diff")
	f.StringArrayVar(&options.Set, "set", nil, "additional values to be merged into the helm command --set flag")
	f.StringArrayVar(&options.Values, "values", nil, "additional value files to be merged into the helm command --values flag")
	f.IntVar(&options.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")
	f.BoolVar(&options.Validate, "validate", false, "validate your manifests against the Kubernetes cluster you are currently pointing at. Note that this requires access to a Kubernetes cluster to obtain information necessary for validating, like the diff of available API versions")
	f.BoolVar(&options.SkipNeeds, "skip-needs", true, `do not automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided. Defaults to true when --include-needs or --include-transitive-needs is not provided`)
	f.BoolVar(&options.IncludeTests, "include-tests", false, "enable the diffing of the helm test hooks")
	f.BoolVar(&options.IncludeNeeds, "include-needs", false, `automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided`)
	f.BoolVar(&options.IncludeTransitiveNeeds, "include-transitive-needs", false, `like --include-needs, but also includes transitive needs (needs of needs). Does nothing when --selector/-l flag is not provided. Overrides exclusions of other selectors and conditions.`)
	f.BoolVar(&options.SkipDiffOnInstall, "skip-diff-on-install", false, "Skips running helm-diff on releases being newly installed on this apply. Useful when the release manifests are too huge to be reviewed, or it's too time-consuming to diff at all")
	f.BoolVar(&options.ShowSecrets, "show-secrets", false, "do not redact secret values in the output. should be used for debug purpose only")
	f.BoolVar(&options.NoHooks, "no-hooks", false, "do not diff changes made by hooks.")
	f.BoolVar(&options.DetailedExitcode, "detailed-exitcode", false, "return a detailed exit code")
	f.BoolVar(&options.StripTrailingCR, "strip-trailing-cr", false, "strip trailing carriage return on input")
	f.IntVar(&options.Context, "context", 0, "output NUM lines of context around changes")
	f.StringVar(&options.Output, "output", "", "output format for diff plugin")
	f.BoolVar(&options.SuppressSecrets, "suppress-secrets", false, "suppress secrets in the output. highly recommended to specify on CI/CD use-cases")
	f.StringArrayVar(&options.Suppress, "suppress", nil, "suppress specified Kubernetes objects in the output. Can be provided multiple times. For example: --suppress KeycloakClient --suppress VaultSecret")
	f.BoolVar(&options.ReuseValues, "reuse-values", false, `Override helmDefaults.reuseValues "helm diff upgrade --install --reuse-values"`)
	f.BoolVar(&options.ResetValues, "reset-values", false, `Override helmDefaults.reuseValues "helm diff upgrade --install --reset-values"`)
	f.StringVar(&options.PostRenderer, "post-renderer", "", `pass --post-renderer to "helm template" or "helm upgrade --install"`)
	f.StringArrayVar(&options.PostRendererArgs, "post-renderer-args", nil, `pass --post-renderer-args to "helm template" or "helm upgrade --install"`)
	f.StringArrayVar(&options.SuppressOutputLineRegex, "suppress-output-line-regex", nil, "a list of regex patterns to suppress output lines from the diff output")

	// Register flags using the registry
	flagRegistry.RegisterFlags(cmd)

	return cmd
}
