# Version Package

## Overview

The `version` package contains version-related constants and utilities used throughout the Helmfile codebase. It was created as a standalone package to avoid import cycles while making version information accessible to all components.

## Contents

This package includes:

- `HelmRequiredVersion`: The minimum required Helm version for Helmfile
- Other version-related constants and utilities

## Usage

Import this package when you need access to version information:

```go
import "github.com/helmfile/helmfile/pkg/version"

func someFunction() {
    // Use version constants
    requiredVersion := version.HelmRequiredVersion

    // Example usage
    fmt.Printf("Helmfile requires Helm version %s or later\n", requiredVersion)
}
```

## Design Considerations

This package is intentionally isolated from other Helmfile packages to prevent import cycles. It should:

- Contain only version-related constants and simple utilities
- Not import other Helmfile packages
- Be kept minimal to serve its specific purpose

When adding new version-related functionality, ensure it belongs here rather than in a more specific package.
