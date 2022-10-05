package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewBuildCmd returns build subcmd
func NewBuildCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	buildOptions := config.NewBuildOptions()

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build all resources from state file",
		RunE: func(cmd *cobra.Command, args []string) error {
			buildImpl := config.NewBuildImpl(globalCfg, buildOptions)
			err := config.NewCLIConfigImpl(buildImpl.GlobalImpl)
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
