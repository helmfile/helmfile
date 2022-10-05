package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewStatusCmd returns status subcmd
func NewStatusCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	statusOptions := config.NewStatusOptions()

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Retrieve status of releases in state file",
		RunE: func(cmd *cobra.Command, args []string) error {
			statusImpl := config.NewStatusImpl(globalCfg, statusOptions)
			err := config.NewCLIConfigImpl(statusImpl.GlobalImpl)
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
	f.IntVar(&statusOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")

	return cmd
}
