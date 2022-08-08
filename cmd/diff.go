package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/spf13/cobra"
)

// NewDiffCmd returns diff subcmd
func NewDiffCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	diffOptions := config.NewDiffOptions()
	diffImpl := config.NewDiffImpl(globalCfg, diffOptions)

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff releases defined in state file",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(diffImpl.GlobalImpl)
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
	f.StringVar(&diffOptions.Args, "args", "", "pass args to helm diff")
	f.StringArrayVar(&diffOptions.Set, "set", []string{}, "additional values to be merged into the command")
	f.StringArrayVar(&diffOptions.Values, "values", []string{}, "additional value files to be merged into the command")
	f.IntVar(&diffOptions.Concurrency, "concurrency", 0, "maximum number of concurrent downloads of release charts")
	f.BoolVar(&diffOptions.Validate, "validate", false, "validate your manifests against the Kubernetes cluster you are currently pointing at. Note that this requires access to a Kubernetes cluster to obtain information necessary for validating, like the diff of available API versions")
	f.BoolVar(&diffOptions.SkipNeeds, "skip-needs", false, `do not automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when when --selector/-l flag is not provided. Defaults to true when --include-needs or --include-transitive-needs is not provided`)
	f.BoolVar(&diffOptions.IncludeTests, "include-tests", false, "enable the diffing of the helm test hooks")
	f.BoolVar(&diffOptions.IncludeNeeds, "include-needs", false, `automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when when --selector/-l flag is not provided`)
	f.BoolVar(&diffOptions.IncludeTransitiveNeeds, "include-transitive-needs", false, `like --include-needs, but also includes transitive needs (needs of needs). Does nothing when when --selector/-l flag is not provided. Overrides exclusions of other selectors and conditions.`)
	f.BoolVar(&diffOptions.SkipDeps, "skip-deps", false, `skip running "helm repo update" and "helm dependency build"`)
	f.BoolVar(&diffOptions.SkipDiffOnInstall, "skip-diff-on-install", false, "Skips running helm-diff on releases being newly installed on this apply. Useful when the release manifests are too huge to be reviewed, or it's too time-consuming to diff at all")
	f.BoolVar(&diffOptions.ShowSecrets, "show-secrets", false, "do not redact secret values in the output. should be used for debug purpose only")
	f.BoolVar(&diffOptions.DetailedExitcode, "detailed-exitcode", false, "return a detailed exit code")
	f.IntVar(&diffOptions.Context, "context", 0, "output NUM lines of context around changes")
	f.StringVar(&diffOptions.Output, "output", "", "output format for diff plugin")
	f.BoolVar(&diffOptions.SuppressSecrets, "suppress-secrets", false, "suppress secrets in the output. highly recommended to specify on CI/CD use-cases")
	f.StringArrayVar(&diffOptions.Suppress, "suppress", []string{}, "suppress specified Kubernetes objects in the output. Can be provided multiple times. For example: --suppress KeycloakClient --suppress VaultSecret")

	return cmd
}
