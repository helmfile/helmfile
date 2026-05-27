package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewSyncCmd returns sync subcmd
func NewSyncCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	syncOptions := config.NewSyncOptions()

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync releases defined in state file",
		RunE: func(cmd *cobra.Command, args []string) error {
			syncImpl := config.NewSyncImpl(globalCfg, syncOptions)
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
	f.StringVar(&syncOptions.SyncArgs, "sync-args", "", "pass args to helm upgrade")
	f.StringArrayVar(&syncOptions.Set, "set", nil, "additional values to be merged into the helm command --set flag")
	f.StringArrayVar(&syncOptions.Values, "values", nil, "additional value files to be merged into the helm command --values flag")
	f.IntVar(&syncOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")
	f.BoolVar(&syncOptions.Validate, "validate", false, "validate your manifests against the Kubernetes cluster you are currently pointing at. Note that this requires access to a Kubernetes cluster to obtain information necessary for validating, like the sync of available API versions")
	f.BoolVar(&syncOptions.SkipNeeds, "skip-needs", true, `do not automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided. Defaults to true when --include-needs or --include-transitive-needs is not provided`)
	f.BoolVar(&syncOptions.SkipCRDs, "skip-crds", false, "if set, no CRDs will be installed on sync. By default, CRDs are installed if not already present")
	f.BoolVar(&syncOptions.IncludeNeeds, "include-needs", false, `automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided`)
	f.BoolVar(&syncOptions.IncludeTransitiveNeeds, "include-transitive-needs", false, `like --include-needs, but also includes transitive needs (needs of needs). Does nothing when --selector/-l flag is not provided. Overrides exclusions of other selectors and conditions.`)
	f.BoolVar(&syncOptions.EnforceNeedsAreInstalled, "enforce-needs-are-installed", false, "enforce that all 'needs' dependencies are installable before applying changes")
	f.BoolVar(&syncOptions.HideNotes, "hide-notes", false, "add --hide-notes flag to helm")
	f.BoolVar(&syncOptions.TakeOwnership, "take-ownership", false, `add --take-ownership flag to helm`)
	f.BoolVar(&syncOptions.SyncReleaseLabels, "sync-release-labels", false, "sync release labels to the target release")
	f.BoolVar(&syncOptions.Wait, "wait", false, `Override helmDefaults.wait setting "helm upgrade --install --wait"`)
	f.BoolVar(&syncOptions.WaitForJobs, "wait-for-jobs", false, `Override helmDefaults.waitForJobs setting "helm upgrade --install --wait-for-jobs"`)
	f.IntVar(&syncOptions.Timeout, "timeout", 0, `Override helmDefaults.timeout in seconds for "helm upgrade --install --timeout" (default 0, which uses helmDefaults.timeout or helm's default if not set)`)
	f.BoolVar(&syncOptions.ReuseValues, "reuse-values", false, `Override helmDefaults.reuseValues "helm upgrade --install --reuse-values"`)
	f.BoolVar(&syncOptions.ResetValues, "reset-values", false, `Override helmDefaults.reuseValues "helm upgrade --install --reset-values"`)
	f.StringVar(&syncOptions.PostRenderer, "post-renderer", "", `pass --post-renderer to "helm template" or "helm upgrade --install"`)
	f.StringArrayVar(&syncOptions.PostRendererArgs, "post-renderer-args", nil, `pass --post-renderer-args to "helm template" or "helm upgrade --install"`)
	f.BoolVar(&syncOptions.SkipSchemaValidation, "skip-schema-validation", false, `pass --skip-schema-validation to "helm template" or "helm upgrade --install"`)
	f.StringVar(&syncOptions.Cascade, "cascade", "", "pass cascade to helm exec, default: background")
	f.StringVar(&syncOptions.TrackMode, "track-mode", "", "Track mode for releases: 'helm' (default), 'helm-legacy' (Helm v4 only), or 'kubedog'")
	f.IntVar(&syncOptions.TrackTimeout, "track-timeout", 0, `Timeout in seconds for kubedog tracking (0 to use default 300s timeout)`)
	f.BoolVar(&syncOptions.TrackLogs, "track-logs", false, "Enable log streaming with kubedog tracking")
	f.BoolVar(&syncOptions.TrackFailOnError, "track-fail-on-error", false, "Fail with non-zero exit code when kubedog tracking fails")
	f.StringVar(&syncOptions.Description, "description", "", `Set description for all releases. If set, overrides descriptions in helmfile.yaml. Will be passed to "helm upgrade --description"`)

	// Diff-related flags for --interactive mode
	f.IntVar(&syncOptions.Context, "context", 0, "output NUM lines of context around changes (interactive preview only)")
	f.StringVar(&syncOptions.DiffOutput, "output", "", "output format for diff plugin (interactive preview only)")
	f.StringVar(&syncOptions.DiffArgs, "diff-args", "", "pass args to helm-diff (interactive preview only)")
	f.StringArrayVar(&syncOptions.Suppress, "suppress", nil, "suppress specified Kubernetes objects in the diff output (interactive preview only). Can be provided multiple times. For example: --suppress KeycloakClient --suppress VaultSecret")
	f.BoolVar(&syncOptions.SuppressSecrets, "suppress-secrets", false, "suppress secrets in the diff output (interactive preview only). highly recommended to specify on CI/CD use-cases")
	f.BoolVar(&syncOptions.ShowSecrets, "show-secrets", false, "do not redact secret values in the diff output (interactive preview only). should be used for debug purpose only")
	f.BoolVar(&syncOptions.NoHooks, "no-hooks", false, "do not diff changes made by hooks (interactive preview only)")
	f.BoolVar(&syncOptions.SuppressDiff, "suppress-diff", false, "suppress diff in the output (interactive preview only). Usable in new installs")
	f.BoolVar(&syncOptions.SkipDiffOnInstall, "skip-diff-on-install", false, "Skips running helm-diff on releases being newly installed on this sync (interactive preview only). Useful when the release manifests are too huge to be reviewed, or it's too time-consuming to diff at all")
	f.BoolVar(&syncOptions.IncludeTests, "include-tests", false, "enable the diffing of the helm test hooks (interactive preview only)")
	f.BoolVar(&syncOptions.DetailedExitcode, "detailed-exitcode", false, "return a non-zero exit code 2 instead of 0 when releases are synced (use --interactive to also see a diff preview)")
	f.BoolVar(&syncOptions.StripTrailingCR, "strip-trailing-cr", false, "strip trailing carriage return on input (interactive preview only)")
	f.StringArrayVar(&syncOptions.SuppressOutputLineRegex, "suppress-output-line-regex", nil, "a list of regex patterns to suppress output lines from diff output (interactive preview only)")

	return cmd
}
