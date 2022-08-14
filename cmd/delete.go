package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewDeleteCmd returns delete subcmd
func NewDeleteCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	deleteOptions := config.NewDeleteOptions()
	deleteImpl := config.NewDeleteImpl(globalCfg, deleteOptions)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "DEPRECATED: delete releases from state file (helm delete)",
		RunE: func(cmd *cobra.Command, args []string) error {
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
	f.StringVar(&deleteOptions.Args, "args", "", "pass args to helm exec")
	f.IntVar(&deleteOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")
	f.BoolVar(&deleteOptions.Purge, "purge", false, "purge releases i.e. free release names and histories")
	f.BoolVar(&deleteOptions.SkipDeps, "skip-deps", false, `skip running "helm repo update" and "helm dependency build"`)

	return cmd
}
