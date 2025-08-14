# Copilot Instructions for Helmfile

## Repository Overview

Helmfile is a declarative spec for deploying Helm charts that manages Kubernetes deployments as code. It provides templating, environment management, and GitOps workflows for Helm chart deployments.

**Key Details:**
- **Language:** Go 1.24.2+
- **Type:** CLI tool / Kubernetes deployment management
- **Size:** Large codebase (~229MB binary, 200+ dependencies)
- **Runtime:** Requires Helm 3.x as external dependency
- **Target Platform:** Linux/macOS/Windows, Kubernetes clusters

## Build and Validation Commands

### Essential Setup
Helmfile requires Helm 3.x as a runtime dependency:
```bash
# Check for Helm dependency (REQUIRED)
helm version  # Must show version 3.x

# Initialize Helm plugins after helmfile installation
./helmfile init  # Installs required helm-diff plugin

# Alternative: Force install without prompts
./helmfile init --force
```

### Build Process
```bash
# Standard build (takes 2-3 minutes due to many dependencies)
make build

# Alternative direct build
go build -o helmfile .

# Build with test tools (required for integration tests, ~1 minute)
make build-test-tools  # Creates diff-yamls and downloads dyff

# Cross-platform builds
make cross
```

**Build Timing:** First build downloads 200+ Go packages and takes 2-3 minutes. Subsequent builds are faster due to module cache. Test tools build is faster (~1 minute).

### Validation Pipeline
Run in this exact order to match CI requirements:

```bash
# 1. Code formatting and linting  
make check           # Run go vet (required - always works)
# Note: make fmt requires gci tool (go install github.com/daixiang0/gci@latest)

# 2. Unit tests (fast, ~30 seconds)
go test -v ./pkg/... -race -p=1

# 3. Integration tests (requires Kubernetes - see Environment Setup)
make integration     # Takes 5-10 minutes, needs minikube/k8s cluster

# 4. E2E tests (optional, needs expect package)
sudo apt-get install expect  # On Ubuntu/Debian
bash test/e2e/helmfile-init/init_linux.sh
```

### Linting Configuration
Uses golangci-lint with configuration in `.golangci.yaml`. Install via:
```bash
# For local development
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.1.6
golangci-lint run
```

**Critical Lint Rules:** staticcheck, errcheck, revive, unused. Fix lint errors before committing.

## Environment Setup Requirements

### Dependencies for Development
```bash
# Required for building/testing
go version            # Must be 1.24.2+
helm version         # Must be 3.x
kubectl version      # For K8s integration

# Required for integration tests
minikube start       # Or other K8s cluster
kustomize version    # v5.2.1+ for some tests
```

### Integration Test Environment
Integration tests require a running Kubernetes cluster:

```bash
# Using minikube (recommended for CI)
minikube start
export KUBECONFIG=$(minikube kubeconfig-path)

# Using kind (alternative)
kind create cluster --name helmfile-test

# Verify cluster access
kubectl cluster-info
```

**Timing:** Integration tests take 5-10 minutes and may fail due to timing issues in resource-constrained environments.

## Project Architecture and Layout

### Core Directory Structure
```
/
├── main.go                    # Entry point - CLI initialization and signal handling
├── cmd/                       # CLI commands (apply, diff, sync, template, etc.)
│   ├── root.go               # Main cobra command setup and global flags  
│   ├── apply.go              # helmfile apply command
│   ├── diff.go               # helmfile diff command
│   └── ...                   # Other subcommands
├── pkg/                       # Core library packages
│   ├── app/                  # Main application logic and execution
│   ├── state/                # Helmfile state management and chart dependencies
│   ├── helmexec/             # Helm execution and command wrapper
│   ├── tmpl/                 # Go template processing and functions
│   ├── environment/          # Environment and values management  
│   └── ...                   # Other core packages
├── test/                      # Test suites
│   ├── integration/          # Integration tests with real K8s clusters
│   ├── e2e/                  # End-to-end user workflow tests
│   └── advanced/             # Advanced feature tests
├── docs/                      # Documentation (mkdocs format)
├── examples/                  # Example helmfile configurations
└── .github/workflows/        # CI/CD pipeline definitions
```

### Key Source Files
- **main.go:** Signal handling, CLI execution entry point
- **cmd/root.go:** Global CLI configuration, error handling, logging setup  
- **pkg/app/app.go:** Main application orchestration, state management
- **pkg/state/state.go:** Helmfile state parsing, release management
- **pkg/helmexec/exec.go:** Helm command execution, version detection

### Configuration Files
- **go.mod/go.sum:** Go dependencies (many cloud providers, k8s libraries)
- **.golangci.yaml:** Linting rules and settings
- **Makefile:** Build targets and development workflows
- **mkdocs.yml:** Documentation generation configuration
- **.github/workflows/ci.yaml:** Complete CI pipeline definition

## Continuous Integration Pipeline

### GitHub Actions Workflow (`.github/workflows/ci.yaml`)
1. **Lint Job:** golangci-lint with custom configuration (~5 minutes)
2. **Test Job:** Unit tests + binary build (~10 minutes)  
3. **Integration Job:** Tests with multiple Helm/Kustomize versions (~15-20 minutes each)
4. **E2E Job:** User workflow validation (~5 minutes)

**Matrix Testing:** CI tests against multiple Helm versions (3.17.x, 3.18.x) and Kustomize versions (5.2.x, 5.4.x).

### Pre-commit Validation Steps
Always run these locally before pushing:
```bash
make check           # Format and vet (required)
go test ./pkg/...    # Unit tests  
make build          # Verify build works
# Note: make fmt requires gci tool: go install github.com/daixiang0/gci@latest
```

### Common CI Failure Causes
- **Linting errors:** Run `golangci-lint run` locally first
- **Integration test timeouts:** K8s cluster setup timing issues
- **Version compatibility:** Ensure Go 1.24.2+ and Helm 3.x
- **Race conditions:** Some tests are sensitive to parallel execution

## Development Gotchas and Known Issues

### Build Issues
- **Long initial build time:** First `make build` downloads 200+ packages (~2-3 minutes)
- **Memory usage:** Large binary size due to embedded dependencies  
- **Git tags:** Build may show version warnings if not on tagged commit
- **Tool dependencies:** `make fmt` requires `gci` tool installation

### Testing Issues  
- **Integration tests require K8s:** Will fail without cluster access
- **Test isolation:** Use `-p=1` flag to avoid race conditions
- **Minikube timing:** May need to wait for cluster ready state
- **Plugin dependencies:** Tests need helm-diff and helm-secrets plugins

### Runtime Requirements
- **Helm dependency:** Always required at runtime, not just build time (available in CI)
- **kubectl access:** Most operations need valid kubeconfig
- **Plugin management:** `helmfile init` must be run after installation

### Common Error Patterns
```bash
# Missing Helm
"helm: command not found" → Install Helm first

# Plugin missing  
"Error: plugin 'diff' not found" → Run helmfile init

# K8s access
"connection refused" → Check kubectl cluster-info

# Permission errors
"permission denied" → Check kubeconfig and cluster access

# Missing tools
"gci: No such file or directory" → go install github.com/daixiang0/gci@latest
```

## Working with the Codebase

### Making Changes
- **Small, focused changes:** Each PR should address single concern
- **Test coverage:** Add unit tests for new pkg/ functionality
- **Integration tests:** Update test-cases/ for new CLI features
- **Documentation:** Update docs/ for user-facing changes

### Key Packages to Understand
- **pkg/app:** Main business logic, start here for feature changes
- **pkg/state:** Helmfile parsing and release orchestration  
- **cmd/:** CLI interface changes and new subcommands
- **pkg/helmexec:** Helm integration and command execution

### Architecture Patterns
- **Dependency injection:** App uses interfaces for testability
- **State management:** Immutable state objects, functional transforms
- **Error handling:** Custom error types with exit codes
- **Plugin system:** Extensible via Helm plugins and Go templates

---

**Trust these instructions:** This information is validated against the current codebase. Only search for additional details if these instructions are incomplete or found to be incorrect for your specific task.