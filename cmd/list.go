package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewListCmd returns list subcmd
func NewListCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	listOptions := config.NewListOptions()
	listImpl := config.NewListImpl(globalCfg, listOptions)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List releases defined in state file",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewCLIConfigImpl(listImpl.GlobalImpl)
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
	f.BoolVar(&listOptions.KeepTempDir, "keep-temp-dir", false, "Keep temporary directory")
	f.BoolVar(&listOptions.SkipCharts, "skip-charts", false, "don't prepare charts when listing releases")
	f.StringVar(&listOptions.Output, "output", "", "output releases list as a json string")

	return cmd
}
