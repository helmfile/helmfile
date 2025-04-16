package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/factory"
)

// NewSyncCmd returns sync subcmd
func NewSyncCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	optionsFactory := factory.NewSyncOptionsFactory()
	options := optionsFactory.CreateOptions().(*config.SyncOptions)
	flagRegistry := optionsFactory.GetFlagRegistry()

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync releases defined in state file",
		PreRun: func(cmd *cobra.Command, args []string) {
			flagRegistry.TransferFlags(cmd, options)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			syncImpl := config.NewSyncImpl(globalCfg, options)
			err := config.NewCLIConfigImpl(syncImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := syncImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(syncImpl)
			return toCLIError(syncImpl.GlobalImpl, a.Sync(syncImpl))
		},
	}

	f := cmd.Flags()
	f.StringVar(&globalCfg.GlobalOptions.Args, "args", "", "pass args to helm sync")
	f.StringVar(&options.SyncArgs, "sync-args", "", "pass args to helm upgrade")
	f.StringArrayVar(&options.Set, "set", nil, "additional values to be merged into the helm command --set flag")
	f.StringArrayVar(&options.Values, "values", nil, "additional value files to be merged into the helm command --values flag")
	f.IntVar(&options.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")
	f.BoolVar(&options.Validate, "validate", false, "validate your manifests against the Kubernetes cluster you are currently pointing at. Note that this requires access to a Kubernetes cluster to obtain information necessary for validating, like the sync of available API versions")
	f.BoolVar(&options.SkipNeeds, "skip-needs", true, `do not automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided. Defaults to true when --include-needs or --include-transitive-needs is not provided`)
	f.BoolVar(&options.IncludeNeeds, "include-needs", false, `automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided`)
	f.BoolVar(&options.IncludeTransitiveNeeds, "include-transitive-needs", false, `like --include-needs, but also includes transitive needs (needs of needs). Does nothing when --selector/-l flag is not provided. Overrides exclusions of other selectors and conditions.`)
	f.BoolVar(&options.HideNotes, "hide-notes", false, "add --hide-notes flag to helm")
	f.BoolVar(&options.TakeOwnership, "take-ownership", false, `add --take-ownership flag to helm`)
	f.BoolVar(&options.SyncReleaseLabels, "sync-release-labels", false, "sync release labels to the target release")
	f.BoolVar(&options.Wait, "wait", false, `Override helmDefaults.wait setting "helm upgrade --install --wait"`)
	f.BoolVar(&options.WaitForJobs, "wait-for-jobs", false, `Override helmDefaults.waitForJobs setting "helm upgrade --install --wait-for-jobs"`)
	f.BoolVar(&options.ReuseValues, "reuse-values", false, `Override helmDefaults.reuseValues "helm upgrade --install --reuse-values"`)
	f.BoolVar(&options.ResetValues, "reset-values", false, `Override helmDefaults.reuseValues "helm upgrade --install --reset-values"`)
	f.StringVar(&options.PostRenderer, "post-renderer", "", `pass --post-renderer to "helm template" or "helm upgrade --install"`)
	f.StringArrayVar(&options.PostRendererArgs, "post-renderer-args", nil, `pass --post-renderer-args to "helm template" or "helm upgrade --install"`)
	f.BoolVar(&options.SkipSchemaValidation, "skip-schema-validation", false, `pass --skip-schema-validation to "helm template" or "helm upgrade --install"`)
	f.StringVar(&options.Cascade, "cascade", "", "pass cascade to helm exec, default: background")

	// Register flags using the registry
	flagRegistry.RegisterFlags(cmd)

	return cmd
}
