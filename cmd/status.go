package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/spf13/cobra"
)

// NewStatusCmd returm build subcmd
func NewStatusCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	statusOptions := config.NewStatusOptions()
	statusImpl := config.NewStatusImpl(globalCfg, statusOptions)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Retrieve status of releases in state file",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(statusImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := statusImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(statusImpl)
			return toCLIError(statusImpl.GlobalImpl, a.Status(statusImpl))
		},
	}

	f := cmd.Flags()
	f.StringVar(&statusOptions.Args, "args", "", "pass args to helm exec")

	return cmd
}
