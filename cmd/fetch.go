package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewFetchCmd returns diff subcmd
func NewFetchCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	fetchOptions := config.NewFetchOptions()
	fetchImpl := config.NewFetchImpl(globalCfg, fetchOptions)

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch charts from state file",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(fetchImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := fetchImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(fetchImpl)
			return toCLIError(fetchImpl.GlobalImpl, a.Fetch(fetchImpl))
		},
	}

	f := cmd.Flags()
	f.IntVar(&fetchOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")
	f.BoolVar(&fetchOptions.SkipDeps, "skip-deps", fetchOptions.SkipDeps, `skip running "helm repo update" and "helm dependency build"`)
	f.StringVar(&fetchOptions.OutputDir, "output-dir", fetchOptions.OutputDir, "directory to store charts (default: temporary directory which is deleted when the command terminates)")

	return cmd
}
