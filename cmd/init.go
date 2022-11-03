package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewInitCmd helmfile checks and installs deps
func NewInitCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	options := config.NewInitOptions()
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the helmfile, includes version checking and installation of helm and plug-ins",
		RunE: func(cmd *cobra.Command, args []string) error {
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
	f := cmd.Flags()
	f.BoolVar(&options.Force, "force", false, "Do not prompt, install dependencies required by helmfile")

	return cmd
}
