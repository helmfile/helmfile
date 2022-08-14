package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/spf13/cobra"
)

// NewDestroyCmd returns destroy subcmd
func NewDestroyCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	destroyOptions := config.NewDestroyOptions()
	destroyImpl := config.NewDestroyImpl(globalCfg, destroyOptions)

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroys and then purges releases",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewCLIConfigImpl(destroyImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := destroyImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(destroyImpl)
			return toCLIError(destroyImpl.GlobalImpl, a.Destroy(destroyImpl))
		},
	}

	f := cmd.Flags()
	f.StringVar(&destroyOptions.Args, "args", "", "pass args to helm exec")
	f.IntVar(&destroyOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")
	f.BoolVar(&destroyOptions.SkipDeps, "skip-deps", false, `skip running "helm repo update" and "helm dependency build"`)

	return cmd
}
