package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewUnittestCmd returns unittest subcmd
func NewUnittestCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	unittestOptions := config.NewUnittestOptions()

	cmd := &cobra.Command{
		Use:   "unittest",
		Short: "Unit test charts from state file using helm-unittest plugin",
		RunE: func(cmd *cobra.Command, args []string) error {
			unittestImpl := config.NewUnittestImpl(globalCfg, unittestOptions)
			err := config.NewCLIConfigImpl(unittestImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := unittestImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(unittestImpl)
			return toCLIError(unittestImpl.GlobalImpl, a.Unittest(unittestImpl))
		},
	}

	f := cmd.Flags()
	f.IntVar(&unittestOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")
	f.StringVar(&globalCfg.GlobalOptions.Args, "args", "", "pass args to helm exec")
	f.StringArrayVar(&unittestOptions.Set, "set", nil, "additional values to be merged into the helm command --set flag")
	f.StringArrayVar(&unittestOptions.Values, "values", nil, "additional value files to be merged into the helm command --values flag")
	f.BoolVar(&unittestOptions.FailFast, "fail-fast", false, "fail fast on the first test failure")
	f.BoolVar(&unittestOptions.Color, "color", false, "enforce colored output even when stdout is not a tty (ignored on Helm 4 due to flag parsing issues)")
	f.BoolVar(&unittestOptions.DebugPlugin, "debug-plugin", false, "enable verbose output from the helm-unittest plugin")
	f.BoolVar(&unittestOptions.SkipNeeds, "skip-needs", true, `do not automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided. Defaults to true when --include-needs or --include-transitive-needs is not provided`)
	f.BoolVar(&unittestOptions.IncludeNeeds, "include-needs", false, `automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided`)
	f.BoolVar(&unittestOptions.IncludeTransitiveNeeds, "include-transitive-needs", false, `like --include-needs, but also includes transitive needs (needs of needs). Does nothing when --selector/-l flag is not provided. Overrides exclusions of other selectors and conditions.`)

	return cmd
}
