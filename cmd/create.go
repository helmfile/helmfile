package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

func NewCreateCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	options := config.NewCreateOptions()
	cmd := &cobra.Command{
		Use:   "create [NAME]",
		Short: "Create a helmfile deployment project scaffold",
		Long: `Create a helmfile deployment project with best-practice directory structure.

Generates:
  - helmfile.yaml          Main configuration with commented examples
  - environments/           Environment-specific value files
  - values/                 Release-specific value files

If NAME is provided, creates the project in a new directory named NAME.
Otherwise, creates the project in the current directory.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				options.Name = args[0]
			}
			createImpl := config.NewCreateImpl(globalCfg, options)
			if err := createImpl.ValidateConfig(); err != nil {
				return err
			}
			a := app.New(createImpl)
			return toCLIError(createImpl.GlobalImpl, a.Create(createImpl))
		},
	}
	f := cmd.Flags()
	f.StringVarP(&options.OutputDir, "output-dir", "o", "", "Output directory (default: NAME or current directory)")
	f.BoolVar(&options.Force, "force", false, "Overwrite existing helmfile.yaml")

	return cmd
}
