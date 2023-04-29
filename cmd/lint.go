package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewLintCmd returns lint subcmd
func NewLintCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	lintOptions := config.NewLintOptions()

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Lint charts from state file (helm lint)",
		RunE: func(cmd *cobra.Command, args []string) error {
			lintImpl := config.NewLintImpl(globalCfg, lintOptions)
			err := config.NewCLIConfigImpl(lintImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := lintImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(lintImpl)
			return toCLIError(lintImpl.GlobalImpl, a.Lint(lintImpl))
		},
	}

	f := cmd.Flags()
	f.IntVar(&lintOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")
	f.StringVar(&globalCfg.GlobalOptions.Args, "args", "", "pass args to helm exec")
	f.StringArrayVar(&lintOptions.Set, "set", nil, "additional values to be merged into the helm command --set flag")
	f.StringArrayVar(&lintOptions.Values, "values", nil, "additional value files to be merged into the helm command --values flag")
	f.BoolVar(&lintOptions.SkipNeeds, "skip-needs", true, `do not automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided. Defaults to true when --include-needs or --include-transitive-needs is not provided`)
	f.BoolVar(&lintOptions.IncludeNeeds, "include-needs", false, `automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided`)
	f.BoolVar(&lintOptions.IncludeTransitiveNeeds, "include-transitive-needs", false, `like --include-needs, but also includes transitive needs (needs of needs). Does nothing when --selector/-l flag is not provided. Overrides exclusions of other selectors and conditions.`)

	return cmd
}
