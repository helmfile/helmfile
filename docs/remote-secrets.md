# Secrets

helmfile can handle secrets using [helm-secrets](https://github.com/jkroepke/helm-secrets) plugin or using remote secrets storage
(everything that package [vals](https://github.com/helmfile/vals) can handle vault, AWS SSM etc)
This section will describe the second use case.

# Remote secrets

This paragraph will describe how to use remote secrets storage (vault, SSM etc) in helmfile

## Fetching single key

To fetch single key from remote secret storage you can use `fetchSecretValue` template function example below

```yaml
# helmfile.yaml

repositories:
  - name: stable
    url: https://charts.helm.sh/stable
---
environments:
  default:
    values:
      - service:
          password: ref+vault://svc/#pass
          login: ref+vault://svc/#login
releases:
  - name: service
    namespace: default
    labels:
      cluster: services
      secrets: vault
    chart: stable/svc
    version: 0.1.0
    values:
      - service:
          login: {{ .Values.service.login | fetchSecretValue }} # this will resolve ref+vault://svc/#pass and fetch secret from vault
          password: {{ .Values.service.password | fetchSecretValue | quote }}
      # - values/service.yaml.gotmpl   # alternatively
```
## Fetching multiple keys
Alternatively you can use `expandSecretRefs` to fetch a map of secrets
```yaml
# values/service.yaml.gotmpl
service:
{{ .Values.service | expandSecretRefs | toYaml | nindent 2 }}
```

This will produce
```yaml
# values/service.yaml
service:
  login: svc-login # fetched from vault
  password: pass

```


## Disabling vals

You can disable the built-in vals processing using environment variables:

### Pass-through mode

Set `HELMFILE_DISABLE_VALS=true` to disable internal vals processing. Any `ref+` values will pass through unchanged, allowing you to validate them with a policy tool such as [conftest](https://www.conftest.dev/) before they are resolved:

```bash
HELMFILE_DISABLE_VALS=true helmfile template | conftest test -
```

### Strict mode

Set `HELMFILE_DISABLE_VALS_STRICT=true` to disable vals and error if any `ref+` values are detected. This is useful when you want to prevent users from using vals references:

```bash
HELMFILE_DISABLE_VALS_STRICT=true helmfile sync
# Error: vals is disabled via HELMFILE_DISABLE_VALS_STRICT environment variable
```

Note: If both are set, strict mode takes precedence.

### Validating ref+ expressions with conftest

You can use `HELMFILE_DISABLE_VALS=true` with [conftest](https://www.conftest.dev/) to validate that all `ref+` expressions conform to your security policy before processing them.

Example rego policy (`policy/vals_refs.rego`):

```rego
package main

allowed_refs := {
    "ref+tfstates3://my-terraform-state/networking/eu-west-2/vpc/vpc_id",
    "ref+tfstates3://my-terraform-state/networking/eu-west-2/vpc/private_subnet_ids",
    "ref+tfstates3://my-terraform-state/platform/eu-west-2/eks/cluster_endpoint",
}

deny[msg] {
    value := input[_]
    startswith(value, "ref+tfstates3://")
    not allowed_refs[value]
    msg := sprintf("ref+ expression references an unapproved tfstates3 URI: %s", [value])
}

deny[msg] {
    value := input[_]
    startswith(value, "ref+")
    not startswith(value, "ref+tfstates3://")
    msg := sprintf("only tfstates3 ref+ expressions are permitted, got: %s", [value])
}
```

Run against your rendered values:

```bash
HELMFILE_DISABLE_VALS=true helmfile template | conftest test -
```
