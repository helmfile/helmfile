# Config Package

## Overview

The `pkg/config` package contains configuration options for various Helmfile commands. These options control the behavior of commands like `apply`, `diff`, `template`, `sync`, and others.

## File Structure

```
pkg/config/
├── options.go            # Base options interfaces and common functionality
├── options_test.go       # Tests for base options
├── global.go             # Global options shared across all commands
├── global_impl.go        # Implementation of global options
├── apply.go              # ApplyOptions implementation for 'apply' command
├── apply_impl.go         # Implementation of ApplyOptions
├── apply_test.go         # Tests for ApplyOptions
├── build.go              # BuildOptions implementation for 'build' command
├── build_impl.go         # Implementation of BuildOptions
├── cache.go              # CacheOptions implementation for 'cache' command
├── cache_impl.go         # Implementation of CacheOptions
├── deps.go               # DepsOptions implementation for 'deps' command
├── deps_impl.go          # Implementation of DepsOptions
├── destroy.go            # DestroyOptions implementation for 'destroy' command
├── destroy_impl.go       # Implementation of DestroyOptions
├── destroy_test.go       # Tests for DestroyOptions
├── diff.go               # DiffOptions implementation for 'diff' command
├── diff_impl.go          # Implementation of DiffOptions
├── diff_test.go          # Tests for DiffOptions
├── fetch.go              # FetchOptions implementation for 'fetch' command
├── fetch_impl.go         # Implementation of FetchOptions
├── fetch_test.go         # Tests for FetchOptions
├── init.go               # InitOptions implementation for 'init' command
├── init_impl.go          # Implementation of InitOptions
├── lint.go               # LintOptions implementation for 'lint' command
├── linit_impl.go         # Implementation of LintOptions
├── lint_test.go          # Tests for LintOptions
├── list.go               # ListOptions implementation for 'list' command
├── list_impl.go          # Implementation of ListOptions
├── list_test.go          # Tests for ListOptions
├── repos.go              # ReposOptions implementation for 'repos' command
├── repos_impl.go         # Implementation of ReposOptions
├── show-dag.go           # ShowDAGOptions implementation for 'show-dag' command
├── show-dag_impl.go      # Implementation of ShowDAGOptions
├── status.go             # StatusOptions implementation for 'status' command
├── status_impl.go        # Implementation of StatusOptions
├── status_test.go        # Tests for StatusOptions
├── sync.go               # SyncOptions implementation for 'sync' command
├── sync_impl.go          # Implementation of SyncOptions
├── sync_test.go          # Tests for SyncOptions
├── template.go           # TemplateOptions implementation for 'template' command
├── template_impl.go      # Implementation of TemplateOptions
├── template_test.go      # Tests for TemplateOptions
├── test.go               # TestOptions implementation for 'test' command
├── test_impl.go          # Implementation of TestOptions
├── write-values.go       # WriteValuesOptions implementation for 'write-values' command
├── write-values_impl.go  # Implementation of WriteValuesOptions
└── common/               # Common option types shared across commands
    ├── bool_flag.go      # Boolean flag implementation
    ├── string_flag.go    # String flag implementation
    └── array_flag.go     # String array flag implementation
```

## Components

- **Options Interfaces**: Base interfaces that define common option behaviors
- **Global Options**: Configuration options shared across all commands
- **Command-specific Options**: Implementations for each command (e.g., `ApplyOptions`, `DiffOptions`)
- **Implementation Classes**: Classes that combine global options with command-specific options
- **Flag Handling**: Each options struct implements the `FlagHandler` interface from the `pkg/flags` package
- **Common Flag Types**: Reusable flag implementations in the `common` subpackage

## Key Features

- **Command Configuration**: Each command has its own options type that controls its behavior
- **Flag Handling**: Options implement the `FlagHandler` interface to receive flag values
- **Default Values**: Options provide sensible defaults that can be overridden
- **Validation**: Some options include validation logic to ensure valid configurations
- **Implementation Pattern**: Each command has both an options struct and an implementation struct that combines global and command-specific options

## Usage

Options objects are typically created by factory methods and populated with values from command-line flags. They are then passed to command implementations to control their behavior.

### Example:

```go
// Create options
opts := config.NewApplyOptions()

// Handle flags
handled := opts.HandleFlag("include-crds", &includeCRDs, true)
if !handled {
    // Flag wasn't recognized
}

// Create implementation with global options
globalOpts := config.NewGlobalImpl(&config.GlobalOptions{})
impl := config.NewApplyImpl(globalOpts, opts)

// Use in command
cmd.Execute(impl)
```

## Testing

All option implementations should have comprehensive tests in this package. Tests should verify:

1. Default values are set correctly
2. Flags are handled properly
3. The boolean return value from `HandleFlag` correctly indicates whether a flag was recognized
4. Option validation works as expected

## Related Packages

- `pkg/flags`: Provides flag registration and handling functionality
- `pkg/factory`: Creates properly configured options for commands
- `pkg/app`: Uses options to control command execution
