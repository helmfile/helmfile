package testcmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/factory"
	"github.com/helmfile/helmfile/pkg/flags"
)

// CommandTestHelper provides utilities for testing commands
type CommandTestHelper struct {
	Cmd      *cobra.Command
	Registry flags.FlagRegistry
	Options  config.Options
}

// TestDiffCmd creates a diff command for testing and returns a helper with its components
func TestDiffCmd() *CommandTestHelper {
	// Create command components
	optionsFactory := factory.NewDiffOptionsFactory()
	options := optionsFactory.CreateOptions().(*config.DiffOptions)
	registry := optionsFactory.GetFlagRegistry()

	// Create command manually
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Diff releases defined in state file",
	}

	// Register flags
	registry.RegisterFlags(cmd)

	// Transfer flags to options
	registry.TransferFlags(cmd, options)

	return &CommandTestHelper{
		Cmd:      cmd,
		Registry: registry,
		Options:  options,
	}
}

// TestApplyCmd creates an apply command for testing and returns a helper with its components
func TestApplyCmd() *CommandTestHelper {
	// Create command components
	optionsFactory := factory.NewApplyOptionsFactory()
	options := optionsFactory.CreateOptions().(*config.ApplyOptions)
	registry := optionsFactory.GetFlagRegistry()

	// Create command manually
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply all resources from state file only when there are changes",
	}

	// Register flags
	registry.RegisterFlags(cmd)

	return &CommandTestHelper{
		Cmd:      cmd,
		Registry: registry,
		Options:  options,
	}
}

// TestTemplateCmd creates a template command for testing and returns a helper with its components
func TestTemplateCmd() *CommandTestHelper {
	// Create command components
	optionsFactory := factory.NewTemplateOptionsFactory()
	options := optionsFactory.CreateOptions().(*config.TemplateOptions)
	registry := optionsFactory.GetFlagRegistry()

	// Create command manually
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Template releases defined in state file",
	}

	// Register flags
	registry.RegisterFlags(cmd)

	return &CommandTestHelper{
		Cmd:      cmd,
		Registry: registry,
		Options:  options,
	}
}

// TestSyncCmd creates a sync command for testing and returns a helper with its components
func TestSyncCmd() *CommandTestHelper {
	// Create command components
	optionsFactory := factory.NewSyncOptionsFactory()
	options := optionsFactory.CreateOptions().(*config.SyncOptions)
	registry := optionsFactory.GetFlagRegistry()

	// Create command manually
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync all resources from state file",
	}

	// Register flags
	registry.RegisterFlags(cmd)

	return &CommandTestHelper{
		Cmd:      cmd,
		Registry: registry,
		Options:  options,
	}
}
