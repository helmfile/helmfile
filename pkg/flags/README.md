# Flags Package

## Overview

The `pkg/flags` package provides utilities for registering, handling, and transferring command-line flags. It serves as the bridge between the command-line interface and the configuration options.

## File Structure

```
pkg/flags/
├── README.md             # This documentation file
├── registry.go           # Core interface and generic implementation
├── registry_mock.go      # Mock implementation for testing
├── registry_diff.go      # Diff-specific registry
├── registry_apply.go     # Apply-specific registry
├── registry_sync.go      # Sync-specific registry
├── registry_template.go  # Template-specific registry
├── registry_lint.go      # Lint-specific registry
├── registry_destroy.go   # Destroy-specific registry
├── registry_fetch.go     # Fetch-specific registry
├── registry_list.go      # List-specific registry
├── registry_status.go    # Status-specific registry
├── registry_test.go      # Tests for registry implementation
├── flag_handler.go       # FlagHandler interface
├── flag_handler_mock.go  # Mock implementation of FlagHandler
├── flag_handler_test.go  # Tests for flag handler implementations
├── flag_value.go         # Generic flag value retrieval functions
└── flag_value_test.go    # Tests for flag value functions
```

## Components

- **FlagHandler Interface**: Defines how components handle flag values with boolean return for success
- **FlagRegistry**: Manages flag registration and transfer
- **Command-specific Registries**: Specialized registries for each command
- **Helper Functions**: Utilities for getting flag values of different types

## Key Features

- **Type Safety**: Helper functions for safely retrieving typed flag values
- **Flag Registration**: Centralized registration of flags to command objects
- **Flag Transfer**: Mechanism to transfer flag values to option objects
- **Flag Existence Checking**: Methods to check if flags are registered or handled
- **Success Indication**: Boolean return values to indicate if flags were successfully handled

## Usage

```go
// Create a registry
registry := flags.NewGenericFlagRegistry()

// Register flags to a command
registry.RegisterFlags(cmd)

// Transfer flags to options
registry.TransferFlags(cmd, opts)

// Check if a flag is registered
if registry.IsFlagRegistered("my-flag") {
    // Do something
}

// Handle a flag with success indication
handled := opts.HandleFlag("flag-name", value, changed)
if !handled {
    // Flag wasn't recognized
}
```

## Testing

The package includes mock implementations for testing:

- `MockFlagHandler`: For testing flag handling logic
- `MockFlagRegistry`: For testing flag registration and transfer

These mocks can be used to verify flag handling behavior without needing real command objects.
