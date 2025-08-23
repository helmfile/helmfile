package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

func NewShowDAGCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	showDAGOptions := config.NewShowDAGOptions()

	cmd := &cobra.Command{
		Use:   "show-dag",
		Short: "It prints a table with 3 columns, GROUP, RELEASE, and DEPENDENCIES. GROUP is the unsigned, monotonically increasing integer starting from 1. All the releases with the same GROUP are deployed concurrently. Everything in GROUP 2 starts being deployed only after everything in GROUP 1 got successfully deployed. RELEASE is the release that belongs to the GROUP. DEPENDENCIES is the list of releases that the RELEASE depends on. It should always be empty for releases in GROUP 1. DEPENDENCIES for a release in GROUP 2 should have some or all dependencies appeared in GROUP 1. It can be \"some\" because Helmfile simplifies the DAGs of releases into a DAG of groups, so that Helmfile always produce a single DAG for everything written in helmfile.yaml, even when there are technically two or more independent DAGs of releases in it.",
		RunE: func(cmd *cobra.Command, args []string) error {
			showDAGImpl := config.NewShowDAGImpl(globalCfg, showDAGOptions)
			err := config.NewCLIConfigImpl(showDAGImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := showDAGImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(showDAGImpl)
			return toCLIError(showDAGImpl.GlobalImpl, a.PrintDAGState(showDAGImpl))
		},
	}
	return cmd
}
