This directory contains a set of Go test source and testdata
to test the helmfile template's rendering result by calling `helmfile build` or `helmfile template` on test input
and comparing the output against the snapshot.

The `testdata` directory is composed of:

- `charts`: The Helm charts used from within test helmfile configs (`snapshpt/*/input.yaml`) as local charts and remote charts
- `snapshot/$NAME/input.yaml`: The input helmfile config for the test case of `$NAME`
- `snapshot/$NAME/output.yaml`: The expected output of the helmfile command
- `snapshot/$NAME/config.yaml`: The snapshot test configuration file. See the `Config` struct defined in `snapshot_test.go` for more information
