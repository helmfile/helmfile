# Common Package

## Overview

The `pkg/common` package provides common utilities, types, and interfaces used throughout the Helmfile application. This package contains shared functionality that doesn't belong to any specific domain but is used by multiple components.

## Current File Structure

```
pkg/common/
├── README.md                    # This documentation file
├── bool_flag.go                 # Boolean flag implementation
├── bool_flag_test.go            # Tests for boolean flag
├── string_flag.go               # String flag implementation
├── string_flag_test.go          # Tests for string flag
├── string_array_flag.go         # String array flag implementation
├── string_array_flag_test.go    # Tests for array flag
├── map_flag.go                  # Map flag implementation
├── map_flag_test.go             # Tests for map flag
```

## Planned Future Implementations

The following files are planned for future development:
```
├── constants.go          # Common constants used throughout the application
├── errors.go             # Common error types and error handling utilities
├── errors_test.go        # Tests for error utilities
├── logging.go            # Logging utilities and interfaces
├── logging_test.go       # Tests for logging utilities
├── types.go              # Common type definitions
├── types_test.go         # Tests for common types
├── utils.go              # General utility functions
└── utils_test.go         # Tests for utility functions
```

## Components

### Currently Implemented

#### Flag Types
- **BoolFlag**: A boolean flag implementation with value tracking
- **StringFlag**: A string flag implementation with value tracking
- **StringArrayFlag**: A string array flag implementation with value tracking
- **MapFlag**: A map flag implementation with value tracking

### Planned for Future Implementation

#### Utilities
- **Constants**: Common constants used throughout the application
- **Error Handling**: Standardized error types and handling
- **Logging**: Consistent logging interfaces and utilities
- **Types**: Common type definitions
- **Utilities**: General utility functions

## Key Features

### Currently Available
- **Flag Implementations**: Reusable flag implementations with value tracking

### Planned Features
- **Error Handling**: Standardized error types and handling
- **Logging**: Consistent logging interfaces and utilities
- **Type Safety**: Common type definitions for consistent usage

## Usage

### Flag Usage

```go
// Create a boolean flag
includeFlag := common.NewBoolFlag(false)

// Set the flag value
includeFlag.Set(true)

// Get the flag value
value := includeFlag.Value()
```

### Future Error Handling (Planned)

```go
// Create a common error
err := common.NewError("operation failed")

// Check if an error is of a specific type
if common.IsNotFoundError(err) {
    // Handle not found error
}
```

### Future Logging (Planned)

```go
// Create a logger
logger := common.NewLogger()

// Log messages
logger.Info("Operation started")
logger.Error("Operation failed: %v", err)
```

## Related Packages

- `pkg/config`: Uses common flag types for option configuration
- `pkg/flags`: Integrates with common flag types for flag handling
- `pkg/app`: Uses common utilities for application functionality
