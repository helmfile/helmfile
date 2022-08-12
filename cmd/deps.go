package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewDepsCmd returns deps subcmd
func NewDepsCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	depsOptions := config.NewDepsOptions()
	depsImpl := config.NewDepsImpl(globalCfg, depsOptions)

	cmd := &cobra.Command{
		Use:   "deps",
		Short: "Update charts based on their requirements",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(depsImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := depsImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(depsImpl)
			return toCLIError(depsImpl.GlobalImpl, a.Deps(depsImpl))
		},
	}

	f := cmd.Flags()
	f.StringVar(&depsOptions.Args, "args", depsOptions.Args, "pass args to helm exec")
	f.BoolVar(&depsOptions.SkipRepos, "skip-deps", depsOptions.SkipRepos, `skip running "helm repo update" and "helm dependency build"`)

	return cmd
}
