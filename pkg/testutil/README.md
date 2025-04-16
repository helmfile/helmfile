# Helmfile Test Utilities

This package provides testing utilities for the Helmfile project, making it easier to write unit tests for Helm-related functionality.

## Overview

The `testutil` package contains:

1. Mock implementations for Helm execution
2. Utility functions for testing

## Components

### Mock Helm Executors

The package provides mock implementations of Helm executors that can be used in tests:

- `V3HelmExec`: A mock that can be configured to simulate Helm 3 behavior
- `VersionHelmExec`: A mock that can be configured with a specific Helm version

```go
// Create a mock for Helm 3
helmExec := testutil.NewV3HelmExec(true)

// Create a mock for a specific Helm version
versionExec := testutil.NewVersionHelmExec("3.8.0")
```

These mocks implement the Helm executor interface but will panic if any unexpected methods are called, making them useful for strict testing scenarios.

### Utility Functions

#### CaptureStdout

Captures stdout output during the execution of a function:

```go
output, err := testutil.CaptureStdout(func() {
    fmt.Println("Hello, world!")
})
// output will contain "Hello, world!\n"
```

This is useful for testing functions that write to stdout.

## Usage Examples

### Testing with V3HelmExec

```go
func TestMyFunction(t *testing.T) {
    // Create a mock Helm executor configured as Helm 3
    helmExec := testutil.NewV3HelmExec(true)

    // Use in your test
    result := myFunctionThatChecksHelmVersion(helmExec)

    // Assert that the result is as expected for Helm 3
    assert.True(t, result)
}
```

### Testing with VersionHelmExec

```go
func TestVersionCompatibility(t *testing.T) {
    // Create a mock with specific version
    helmExec := testutil.NewVersionHelmExec("3.7.1")

    // Test version comparison
    assert.True(t, helmExec.IsVersionAtLeast("3.7.0"))
    assert.False(t, helmExec.IsVersionAtLeast("3.8.0"))
}
```

### Capturing Output

```go
func TestOutputFunction(t *testing.T) {
    output, err := testutil.CaptureStdout(func() {
        MyFunctionThatPrintsOutput()
    })

    assert.NoError(t, err)
    assert.Contains(t, output, "Expected output")
}
```

## Contributing

When adding new test utilities, please ensure they are well-documented and include appropriate tests.
