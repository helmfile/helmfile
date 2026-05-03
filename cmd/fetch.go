package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/state"
)

// NewFetchCmd returns fetch subcmd
func NewFetchCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	fetchOptions := config.NewFetchOptions()

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch charts from state file",
		Long: `Fetch downloads all charts referenced in the Helmfile state.

Useful for air-gapped environments: download charts with --output-dir and --write-output,
then transfer the output directory and the generated helmfile.yaml to the air-gapped environment.

The --write-output flag requires a single helmfile state file specified with -f.
It fails if the input resolves to multiple state files (e.g. a directory or a helmfile
with nested helmfiles: entries).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fetchImpl := config.NewFetchImpl(globalCfg, fetchOptions)
			err := config.NewCLIConfigImpl(fetchImpl.GlobalImpl)
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
	f.StringVar(&fetchOptions.OutputDir, "output-dir", "", "directory to store charts (default: temporary directory which is deleted when the command terminates)")
	f.StringVar(&fetchOptions.OutputDirTemplate, "output-dir-template", state.DefaultFetchOutputDirTemplate, "go text template for generating the output directory. Available fields: {{ .OutputDir }}, {{ .ChartName }}, {{ .Release.* }}, {{ .Environment.Name }}, {{ .Environment.KubeContext }}, {{ .Environment.Values.* }}")
	f.BoolVar(&fetchOptions.WriteOutput, "write-output", false, "write a helmfile.yaml to stdout with chart references updated to local chart paths; requires --output-dir and a single helmfile (use -f)")

	return cmd
}
