I need to create a comprehensive README.md for the pkg/factory package, including all files in the directory structure.

Action: DirectAnswer

# Factory Package

## Overview

The `pkg/factory` package provides factory methods for creating various components used throughout the Helmfile application. This package is responsible for wiring together different parts of the application and providing a clean API for component creation.

## File Structure

```
pkg/factory/
├── README.md             # This documentation file
├── factory.go            # Core factory interface and implementation
├── factory_test.go       # Tests for factory implementation
├── options.go            # Factory methods for creating options
├── options_test.go       # Tests for options factory methods
├── command.go            # Factory methods for creating commands
├── command_test.go       # Tests for command factory methods
├── helmexec.go           # Factory methods for Helm executor
├── helmexec_test.go      # Tests for Helm executor factory
├── environment.go        # Environment factory methods
├── environment_test.go   # Tests for environment factory
├── state.go              # State factory methods
├── state_test.go         # Tests for state factory
└── mock/                 # Mock implementations for testing
    ├── factory.go        # Mock factory implementation
    └── options.go        # Mock options for testing
```

## Components

- **Factory Interface**: Defines methods for creating various components
- **Factory Implementation**: Concrete implementation of the factory interface
- **Options Factory**: Methods for creating command options
- **Command Factory**: Methods for creating command implementations
- **Helm Executor Factory**: Methods for creating Helm execution components
- **Environment Factory**: Methods for creating environment components
- **State Factory**: Methods for creating state components

## Key Features

- **Dependency Injection**: Provides a clean way to inject dependencies
- **Component Creation**: Centralizes the creation of complex components
- **Configuration**: Handles configuration of components
- **Testing Support**: Provides mock implementations for testing

## Usage

```go
// Create a factory
f := factory.NewFactory()

// Create options
applyOpts := f.NewApplyOptions()

// Create a command
applyCmd := f.NewApplyCommand(applyOpts)

// Execute the command
err := applyCmd.Execute()
```

## Testing Options

The options tests in this package are **only for testing purposes**. They are not intended to be used in production code.

### Important Notes:

- **Primary Options Testing**: The primary location for testing options should be in the `pkg/config` package, not here.
- **Test Fixtures**: The options in this package are test fixtures to facilitate testing of the factory methods.
- **No Production Use**: These test options should not be used in production code or referenced outside of tests.

## Best Practices

When working with options:

1. Implement and test option functionality in `pkg/config`
2. Use the factory methods to create properly configured options in production code
3. Only use the test options in this package for testing factory methods

## Related Packages

- `pkg/config`: Contains the actual implementation of options and should be the primary location for options tests
- `pkg/flags`: Contains flag handling functionality used by options
- `pkg/app`: Uses factory to create components for application execution
