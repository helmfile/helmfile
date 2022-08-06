package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/spf13/cobra"
)

// NewChartsCmd returns charts subcmd
func NewChartsCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	chartsOptions := config.NewChartsOptions()
	chartsImpl := config.NewChartsImpl(globalCfg, chartsOptions)

	cmd := &cobra.Command{
		Use:   "charts",
		Short: "DEPRECATED: sync releases from state file (helm upgrade --install)",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(chartsImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := chartsImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(chartsImpl)
			return toCLIError(chartsImpl.GlobalImpl, a.DeprecatedSyncCharts(chartsImpl))
		},
	}

	f := cmd.Flags()
	f.StringVar(&chartsOptions.Args, "args", chartsOptions.Args, "pass args to helm exec")
	f.StringArrayVar(&chartsOptions.Set, "set", chartsOptions.Set, "additional values to be merged into the command")
	f.StringArrayVar(&chartsOptions.Values, "values", chartsOptions.Values, "additional value files to be merged into the command")
	f.IntVar(&chartsOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")

	return cmd
}
