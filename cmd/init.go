package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewInitCmd helmfile checks and installs deps
func NewInitCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Checks and installs deps",
		RunE: func(cmd *cobra.Command, args []string) error {
			options := config.NewInitOptions()
			initImpl := config.NewInitImpl(globalCfg, options)
			err := config.NewCLIConfigImpl(initImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := initImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(initImpl)
			return toCLIError(initImpl.GlobalImpl, a.Init(initImpl))
		},
	}

	return cmd
}
