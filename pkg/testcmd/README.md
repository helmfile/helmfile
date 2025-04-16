# Helmfile Test Command Package

## Overview

The `testcommand` package provides utilities for testing Helmfile commands in a controlled environment. This package simplifies the creation and configuration of command objects for testing purposes, allowing developers to verify command behavior without executing the full application.

## Components

### CommandTestHelper

The core structure that encapsulates the components needed for testing commands:

- `Cmd`: The Cobra command instance
- `Registry`: Flag registry for managing command flags
- `Options`: Command-specific options

### Available Test Commands

The package provides helper functions to create test instances of the following Helmfile commands:

- **TestDiffCmd()**: Creates a test instance of the `diff` command
- **TestApplyCmd()**: Creates a test instance of the `apply` command
- **TestTemplateCmd()**: Creates a test instance of the `template` command
- **TestSyncCmd()**: Creates a test instance of the `sync` command

## Usage

```go
import (
    "testing"
    "github.com/helmfile/helmfile/pkg/testcommand"
)

func TestMyDiffCommand(t *testing.T) {
    // Create a test diff command
    helper := testcommand.TestDiffCmd()

    // Access the command components
    cmd := helper.Cmd
    options := helper.Options.(*config.DiffOptions)

    // Set up test flags
    cmd.Flags().Set("concurrency", "5")

    // Test command behavior
    // ...
}
```

## Implementation Details

Each test command function:

1. Creates an options factory for the specific command
2. Instantiates the command options
3. Gets the flag registry
4. Creates a Cobra command instance
5. Registers the appropriate flags
6. Returns a helper with all components

For the `diff` command, flag values are automatically transferred to the options object.

## Notes

This package is intended for testing purposes only and should not be used in production code.
