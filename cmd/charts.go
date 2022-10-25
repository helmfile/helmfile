package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewChartsCmd returns charts subcmd
func NewChartsCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	chartsOptions := config.NewChartsOptions()

	cmd := &cobra.Command{
		Use:   "charts",
		Short: "DEPRECATED: sync releases from state file (helm upgrade --install)",
		RunE: func(cmd *cobra.Command, args []string) error {
			chartsImpl := config.NewChartsImpl(globalCfg, chartsOptions)
			err := config.NewCLIConfigImpl(chartsImpl.GlobalImpl)
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
	f.StringVar(&globalCfg.GlobalOptions.Args, "args", "", "pass args to helm exec")
	f.StringArrayVar(&chartsOptions.Set, "set", nil, "additional values to be merged into the command")
	f.StringArrayVar(&chartsOptions.Values, "values", nil, "additional value files to be merged into the command")
	f.IntVar(&chartsOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")

	return cmd
}
