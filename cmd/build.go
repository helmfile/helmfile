package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/spf13/cobra"
)

// NewBuildCmd returns build subcmd
func NewBuildCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	buildOptions := config.NewBuildOptions()
	buildImpl := config.NewBuildImpl(globalCfg, buildOptions)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build all resources from state file only when there are changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(buildImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := buildImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(buildImpl)
			return toCLIError(buildImpl.GlobalImpl, a.PrintState(buildImpl))
		},
	}

	f := cmd.Flags()
	f.BoolVar(&buildOptions.EmbedValues, "embed-values", false, "Read all the values files for every release and embed into the output helmfile.yaml")

	return cmd
}
