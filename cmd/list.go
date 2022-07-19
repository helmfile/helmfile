package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/spf13/cobra"
)

// NewListCmd returm build subcmd
func NewListCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	listOptions := config.NewListOptions()
	listImpl := config.NewListImpl(globalCfg, listOptions)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List releases defined in state file",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(listImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := listImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(listImpl)
			return toCLIError(listImpl.GlobalImpl, a.ListReleases(listImpl))
		},
	}

	f := cmd.Flags()
	f.BoolVar(&listOptions.KeepTempDir, "keep-temp-dir", listOptions.KeepTempDir, "Keep temporary directory")
	f.StringVar(&listOptions.Output, "output", listOptions.Output, "output releases list as a json string")

	return cmd
}
