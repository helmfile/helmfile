# Experimental Features

This document describes the experimental features that are available in Helmfile.

Any experimental feature may be removed or changed in a future release without notice.

Enable experimental features with the environment variable:

```bash
# Enable all experimental features
export HELMFILE_EXPERIMENTAL=true

# Enable a specific feature
export HELMFILE_EXPERIMENTAL=explicit-selector-inheritance
```

## explicit-selector-inheritance

By default, CLI selectors (e.g., `helmfile -l name=myapp sync`) are inherited by sub-helmfiles. This experimental feature changes the behavior so that sub-helmfiles without explicit `selectors` do **not** inherit selectors from their parent or the CLI.

When enabled:
* Sub-helmfiles without `selectors` do not inherit parent/CLI selectors
* Use `selectorsInherited: true` on a sub-helmfile to explicitly opt into inheriting selectors
* `selectors: []` selects all releases (same as current behavior)

See [Selectors and needs](releases.md#selectors) for detailed examples.

## HCL helmfile-values-file support

HCL language is supported for environment values files (`.hcl` suffix). This was introduced as experimental in PR #1423 and is now a stable feature. See [Environments](environments.md#hcl-specifications) for details.
