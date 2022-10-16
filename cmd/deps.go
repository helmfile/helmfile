package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewDepsCmd returns deps subcmd
func NewDepsCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	depsOptions := config.NewDepsOptions()

	cmd := &cobra.Command{
		Use:   "deps",
		Short: "Update charts based on their requirements",
		RunE: func(cmd *cobra.Command, args []string) error {
			depsImpl := config.NewDepsImpl(globalCfg, depsOptions)
			err := config.NewCLIConfigImpl(depsImpl.GlobalImpl)
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
	f.StringVar(&depsOptions.Args, "args", "", "pass args to helm exec")
	f.BoolVar(&depsOptions.SkipRepos, "skip-deps", false, `skip running "helm repo update" and "helm dependency build"`)
	f.IntVar(&depsOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")

	return cmd
}
