# Paths Overview

Using manifest files in conjunction with command line argument can be a bit confusing.

A few rules to clear up this ambiguity:

- Absolute paths are always resolved as absolute paths
- Relative paths referenced *in* the helmfile manifest itself are relative to that manifest
- Relative paths referenced on the command line are relative to the current working directory the user is in

### Examples

There are several examples that we can go through in the [`/examples`](../examples/) folder which demonstrate this.

**Local Execution**

This is an example of a Helmfile manifest referencing a local value directly.

Indirect:
```
helmfile -f examples/deployments/local/charts.yaml sync
```

Direct:
```
cd examples/deployments/local/
helmfile sync
```

**Relative Paths in Helmfile**

This is an example of a Helmfile manifest using relative paths for values.

Indirect:
```
helmfile -f examples/deployments/dev/charts.yaml sync
```

Direct:
```
cd examples/deployments/dev/
helmfile sync
```

**Relative Paths in Helmfile w/ --values overrides**

This is an example of a Helmfile manifest using relative paths for values including an additional `--values` from the command line.

NOTE: The `--values` is resolved relative to the CWD of the terminal *not* the Helmfile manifest.  You can see this with the `replicas` being adjusted to 3 now for the deployment.

Indirect:
```
helmfile -f examples/deployments/dev/charts.yaml sync --values values/replica-values.yaml
```

Direct:
```
cd examples/deployments/dev/
helmfile sync --values ../../values/replica-values.yaml
```
