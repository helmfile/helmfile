package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewDiffCmd returns diff subcmd.
func NewDiffCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	diffOptions := config.NewDiffOptions()

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff releases defined in state file",
		RunE: func(cmd *cobra.Command, args []string) error {
			diffImpl := config.NewDiffImpl(globalCfg, diffOptions)
			err := config.NewCLIConfigImpl(diffImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := diffImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(diffImpl)
			return toCLIError(diffImpl.GlobalImpl, a.Diff(diffImpl))
		},
	}

	// Common surface shared with `helmfile doctor`.
	bindCommonDiffFlags(cmd.Flags(), diffOptions, &globalCfg.GlobalOptions.Args)

	// Diff-specific flags (defaults/help differ from doctor).
	f := cmd.Flags()
	f.BoolVar(&diffOptions.ShowSecrets, "show-secrets", false, "do not redact secret values in the output. should be used for debug purpose only")
	f.BoolVar(&diffOptions.DetailedExitcode, "detailed-exitcode", false, "return a detailed exit code")
	f.IntVar(&diffOptions.Context, "context", 0, "output NUM lines of context around changes")
	f.StringVar(&diffOptions.Output, "output", "", "output format for diff plugin")

	return cmd
}
