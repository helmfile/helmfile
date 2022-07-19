package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/spf13/cobra"
)

// NewReposCmd returm build subcmd
func NewReposCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	reposOptions := config.NewReposOptions()
	reposImpl := config.NewReposImpl(globalCfg, reposOptions)

	cmd := &cobra.Command{
		Use:   "repos",
		Short: "Repos releases defined in state file",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(reposImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := reposImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(reposImpl)
			return toCLIError(reposImpl.GlobalImpl, a.Repos(reposImpl))
		},
	}

	f := cmd.Flags()
	f.StringVar(&reposOptions.Args, "args", reposOptions.Args, "pass args to helm exec")

	return cmd
}
