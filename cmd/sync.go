package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/spf13/cobra"
)

// NewSyncCmd returns sync subcmd
func NewSyncCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	syncOptions := config.NewSyncOptions()
	syncImpl := config.NewSyncImpl(globalCfg, syncOptions)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync releases defined in state file",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(syncImpl.GlobalImpl)
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
	f.StringVar(&syncOptions.Args, "args", "", "pass args to helm sync")
	f.StringArrayVar(&syncOptions.Set, "set", nil, "additional values to be merged into the command")
	f.StringArrayVar(&syncOptions.Values, "values", nil, "additional value files to be merged into the command")
	f.IntVar(&syncOptions.Concurrency, "concurrency", 0, "maximum number of concurrent downloads of release charts")
	f.BoolVar(&syncOptions.Validate, "validate", false, "validate your manifests against the Kubernetes cluster you are currently pointing at. Note that this requires access to a Kubernetes cluster to obtain information necessary for validating, like the sync of available API versions")
	f.BoolVar(&syncOptions.SkipNeeds, "skip-needs", false, `do not automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when when --selector/-l flag is not provided. Defaults to true when --include-needs or --include-transitive-needs is not provided`)
	f.BoolVar(&syncOptions.SkipCRDs, "skip-crds", false, "if set, no CRDs will be installed on sync. By default, CRDs are installed if not already present")
	f.BoolVar(&syncOptions.IncludeNeeds, "include-needs", false, `automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when when --selector/-l flag is not provided`)
	f.BoolVar(&syncOptions.IncludeTransitiveNeeds, "include-transitive-needs", false, `like --include-needs, but also includes transitive needs (needs of needs). Does nothing when when --selector/-l flag is not provided. Overrides exclusions of other selectors and conditions.`)
	f.BoolVar(&syncOptions.SkipDeps, "skip-deps", false, `skip running "helm repo update" and "helm dependency build"`)
	f.BoolVar(&syncOptions.Wait, "wait", false, `Override helmDefaults.wait setting "helm upgrade --install --wait"`)
	f.BoolVar(&syncOptions.WaitForJobs, "wait-for-jobs", false, `Override helmDefaults.waitForJobs setting "helm upgrade --install --wait-for-jobs"`)

	return cmd
}
