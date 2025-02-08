// TODO: Remove this function once Helmfile v0.x
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewDeleteCmd returns delete subcmd
func NewDeleteCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	deleteOptions := config.NewDeleteOptions()

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "DEPRECATED: delete releases from state file (helm delete)",
		RunE: func(cmd *cobra.Command, args []string) error {
			deleteImpl := config.NewDeleteImpl(globalCfg, deleteOptions)
			err := config.NewCLIConfigImpl(deleteImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := deleteImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(deleteImpl)
			return toCLIError(deleteImpl.GlobalImpl, a.Delete(deleteImpl))
		},
	}

	f := cmd.Flags()
	f.StringVar(&globalCfg.GlobalOptions.Args, "args", "", "pass args to helm exec")
	f.StringVar(&deleteOptions.Cascade, "cascade", "", "pass cascade to helm exec, default: background")
	f.IntVar(&deleteOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")
	f.BoolVar(&deleteOptions.Purge, "purge", false, "purge releases i.e. free release names and histories")
	f.BoolVar(&deleteOptions.SkipCharts, "skip-charts", false, "don't prepare charts when deleting releases")
	f.BoolVar(&deleteOptions.DeleteWait, "deleteWait", false, `override helmDefaults.wait setting "helm uninstall --wait"`)
	f.IntVar(&deleteOptions.DeleteTimeout, "deleteTimeout", 300, `time in seconds to wait for helm uninstall, default: 300`)

	return cmd
}
