# AGENTS.md

## Build and Test Commands

### Essential Setup
```bash
# Check Go version (requires 1.24.2+)
go version

# Check Helm dependency (required at runtime)
helm version  # Must show version 3.x

# Install gci tool for import formatting
go install github.com/daixiang0/gci@latest
```

### Build Commands
```bash
# Standard build
make build

# Direct build
go build -o helmfile .

# Cross-platform builds
make cross

# Build test tools (required for integration tests)
make build-test-tools
```

### Linting and Formatting
```bash
# Run go vet (always available)
make check

# Format code with gci
make fmt

# Run golangci-lint
golangci-lint run
```

### Testing Commands
```bash
# Run all unit tests
make test

# Run specific test package
go test -v ./pkg/app/...

# Run single test function
go test -v ./pkg/app/... -run TestSpecificFunction

# Run tests with coverage
go test -v ./pkg/... -coverprofile cover.out -race -p=1
go tool cover -func cover.out

# Run integration tests (requires Kubernetes cluster)
make integration
```

## Code Style Guidelines

### Imports
Import ordering (enforced by gci):
1. Standard library (stdlib)
2. Default (third-party)
3. Local (github.com/helmfile/helmfile prefix)

Always use aliases for common stdlib packages to avoid conflicts:
```go
import (
    goContext "context"  // Alias to avoid naming conflicts
    goruntime "runtime"
    "fmt"
    "os"

    "github.com/helmfile/helmfile/pkg/app"
)
```

### Formatting
- Use `go fmt` for standard formatting
- Use `gci` for import organization: `gci write --skip-generated -s standard -s default -s 'prefix(github.com/helmfile/helmfile)' .`
- Run `make fmt` before committing (requires gci installation)

### Types
- Exported types use PascalCase: `type App struct`
- Private types use PascalCase: `type helmKey struct`
- Interface names should describe behavior: `type Interface interface`
- Use `any` instead of `interface{}`

### Naming Conventions
- Exported functions/variables: PascalCase
- Private functions/variables: camelCase
- Constants: PascalCase
- Test functions: TestXxx with descriptive names
- Package names: lowercase, single word when possible
- Error variables: ErrXxx

### Error Handling
- Use fmt.Errorf with %w for wrapping errors
- Check errors explicitly; never ignore
- Use custom error types in pkg/errors/ when appropriate
- Return error as last return value
```go
if err != nil {
    return fmt.Errorf("failed to parse helm version '%s': %w", version, err)
}
```

### Testing
- Use testify/assert for assertions: `assert.Equal(t, expected, actual)`
- Use testify/require for critical assertions: `require.NoError(t, err)`
- Test files: *_test.go
- Use table-driven tests for multiple scenarios
- Run tests with -race flag: `-race -p=1`
- Use test helper packages: pkg/testutil, pkg/testhelper
```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {
            name:  "valid input",
            input: "test",
            want:  "result",
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FunctionUnderTest(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Logging
- Use zap.SugaredLogger: `logger.Infof("message")`
- Logger injected via dependency injection
- Don't use fmt.Print for production logging

### Comments
- Exported functions must have comments
- Comments should be complete sentences with capital first letter
- Use // for single-line comments
- Use /* */ for package documentation

### Linting Configuration (from .golangci.yaml)
Key enabled linters:
- errcheck: Check unhandled errors
- staticcheck: Advanced static analysis
- revive: Fast linter
- govet: Go vet checks
- ineffassign: Detect ineffectual assignments
- misspell: Spell checking
- unused: Detect unused code

Important thresholds:
- Max function length: 280 lines
- Max statements per function: 140
- Max cognitive complexity: 110
- Max naked return lines: 50
- Line length: 120 characters

### Structure
- Package main: Entry point only (main.go)
- cmd/: CLI commands using cobra
- pkg/: Core library code organized by domain
- test/: Integration and E2E tests
- Use dependency injection for testability
- Prefer composition over inheritance

### Critical Rules
1. Always handle errors
2. Run `make check` before committing
3. Run `golangci-lint run` and fix all issues
4. Write tests for new pkg/ functionality
5. Update docs/ for user-facing changes
6. Follow declarative design principles (desired state in config, operational via flags)

### Common Issues
- First build downloads 200+ packages (2-3 minutes)
- Integration tests require Kubernetes cluster (minikube/kind)
- Make fmt requires gci installation
- Use -p=1 for tests to avoid race conditions
- Always initialize Helm plugins with `helmfile init` after installation
