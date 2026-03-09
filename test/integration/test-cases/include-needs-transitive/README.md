# Test Case: include-needs vs include-transitive-needs

This test case validates the fix for issue #1003.

## Issue
`--include-needs` flag was incorrectly including transitive dependencies when it should only include direct dependencies.

## Expected Behavior

### --include-needs
When selecting `release3` with `--include-needs`:
- **Should include**: `release2` (direct dependency) and `release3` (selected release)
- **Should NOT include**: `release1` (transitive dependency of release3 through release2)

### --include-transitive-needs
When selecting `release3` with `--include-transitive-needs`:
- **Should include**: `release1`, `release2`, and `release3` (all dependencies in the chain)

## Dependency Chain
```
release1 (no dependencies)
    ↑
release2 (needs release1)
    ↑
release3 (needs release2)
```

## Tests Performed
1. `helmfile template --include-needs` - verifies only direct dependencies are included
2. `helmfile template --include-transitive-needs` - verifies all dependencies are included
3. `helmfile lint --include-needs` - verifies consistency across commands
4. `helmfile diff --include-needs` - verifies consistency across commands

## References
- Issue: https://github.com/helmfile/helmfile/issues/1003
