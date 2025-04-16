# Run Struct

## Overview

The `Run` struct is a core component of Helmfile's execution model, responsible for orchestrating the preparation, execution, and cleanup phases of Helmfile commands. It provides a consistent workflow for all Helmfile operations while maintaining separation of concerns.

## Core Mechanics

### The Preparation-Execution-Cleanup Pattern

At the heart of the `Run` package is the `withPreparedCharts` method, which implements a functional programming pattern to standardize how Helmfile commands are executed:

```go
func (r *Run) withPreparedCharts(helmfileCommand string, opts state.ChartPrepareOptions, f func()) error {
    // Preparation phase
    // ...

    // Execution phase
    f()

    // Cleanup phase
    // ...
}
```

This pattern consists of three distinct phases:

1. **Preparation Phase**:
   - Validates the current state
   - Syncs repositories if needed
   - Creates temporary directories for chart downloads
   - Triggers global prepare events
   - Downloads and processes charts

2. **Execution Phase**:
   - Executes the command-specific logic provided as a function parameter
   - Command implementations are decoupled from chart preparation

3. **Cleanup Phase**:
   - Triggers global cleanup events
   - Handles resource cleanup

### Usage Example

Commands like `diff`, `apply`, or `template` use this pattern by providing their specific implementation as a function:

```go
// Example: How the diff command uses withPreparedCharts
err := run.withPreparedCharts("diff", opts, func() {
    msg, matched, affected, errs = a.diff(run, c)
})
```

## Benefits

- **Consistent Workflow**: All commands follow the same preparation, execution, and cleanup flow
- **Separation of Concerns**: Chart preparation logic is centralized and reused across commands
- **Resource Management**: Temporary resources are properly created and cleaned up
- **Extensibility**: New commands can be added by implementing their specific logic without duplicating preparation code

## Key Components

- **ReleaseToChart**: Maps release identifiers to their prepared chart paths
- **Chart Preparation**: Handles downloading, modifying, and preparing charts for use
- **Repository Synchronization**: Ensures all required Helm repositories are available
- **Event Triggers**: Executes global prepare and cleanup events at appropriate times

## Implementation Details

The `Run` struct maintains state throughout the execution lifecycle, ensuring that charts are prepared only once and that all commands have access to the prepared charts through the `ReleaseToChart` map.

Commands can focus on their specific implementation while relying on the `withPreparedCharts` method to handle all the common preparation and cleanup tasks.
