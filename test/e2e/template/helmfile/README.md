This directory contains a set of Go test source and testdata
to test the helmfile template's rendering result by calling `helmfile build` or `helmfile template` on test input
and comparing the output against the snapshot.

The `testdata` directory is composed of:

- `charts`: The Helm charts used from within test helmfile configs (`snapshpt/*/input.yaml`) as local charts and remote charts
- `snapshot/$NAME/input.yaml`: The input helmfile config for the test case of `$NAME`
- `snapshot/$NAME/output.yaml`: The expected output of the helmfile command
- `snapshot/$NAME/config.yaml`: The snapshot test configuration file. See the `Config` struct defined in `snapshot_test.go` for more information
- `snapshot/$NAME/chart`: Optional chart fixture local to the test case, referenced from the input as `./chart/`. Unlike `charts`, it is never packaged/pushed to the test registry, so it may hold directories without `Chart.yaml` (raw manifests that must go through chartify)
