package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

func NewUnittestCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	unittestOptions := config.NewUnittestOptions()

	cmd := &cobra.Command{
		Use:   "unittest",
		Short: "Run unit tests for charts",
		Long:  "Run helm-unittest on releases defined in state file",
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
	f.StringArrayVar(&unittestOptions.Values, "values", nil, "additional value files to be merged into the helm command --values flag")
	f.IntVar(&unittestOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")
	f.BoolVar(&unittestOptions.FailFast, "fail-fast", false, "fail fast on the first test failure")
	f.BoolVar(&unittestOptions.Color, "color", false, "output with color")
	f.BoolVar(&unittestOptions.DebugPlugin, "debug-plugin", false, "output plugin debug information")
	f.StringArrayVar(&unittestOptions.UnittestArgs, "unittest-args", nil, "additional arguments to pass to helm unittest")
	f.BoolVar(&unittestOptions.SkipNeeds, "skip-needs", true, `do not automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided`)
	f.BoolVar(&unittestOptions.IncludeNeeds, "include-needs", false, `automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided`)
	f.BoolVar(&unittestOptions.IncludeTransitiveNeeds, "include-transitive-needs", false, `like --include-needs, but also includes transitive needs (needs of needs). Does nothing when --selector/-l flag is not provided. Overrides exclusions of other selectors and conditions.`)

	return cmd
}
