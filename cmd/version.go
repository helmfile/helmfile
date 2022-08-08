package cmd

import (
	"fmt"

	"github.com/helmfile/helmfile/pkg/app/version"
	"github.com/spf13/cobra"
)

// NewVersionCmd returns version subcmd
func NewVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show the version for Helmfile.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Helmfile version " + version.GetVersion())
			return nil
		},
	}

	return cmd
}
