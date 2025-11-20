package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewPrintEnvCmd returns print-env subcmd
func NewPrintEnvCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	printEnvOptions := config.NewPrintEnvOptions()

	cmd := &cobra.Command{
		Use:   "print-env",
		Short: "Print parsed environment configuration including values and secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvImpl := config.NewPrintEnvImpl(globalCfg, printEnvOptions)
			err := config.NewCLIConfigImpl(printEnvImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := printEnvImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(printEnvImpl)
			return toCLIError(printEnvImpl.GlobalImpl, a.PrintEnv(printEnvImpl))
		},
	}

	f := cmd.Flags()
	f.StringVar(&printEnvOptions.Output, "output", "yaml", "output format: yaml or json")

	return cmd
}
