# Environments

## Environment

When you want to customize the contents of `helmfile.yaml` or `values.yaml` files per environment, use this feature.

You can define as many environments as you want under `environments` in `helmfile.yaml`.
`environments` section should be separated from `releases` with `---`.

The environment name defaults to `default`, that is, `helmfile sync` implies the `default` environment.
The selected environment name can be referenced from `helmfile.yaml` and `values.yaml.gotmpl` by `{{ .Environment.Name }}`.

If you want to specify a non-default environment, provide a `--environment NAME` flag to `helmfile` like `helmfile --environment production sync`.

The below example shows how to define a production-only release:

```yaml
environments:
  default:
  production:

---

releases:
- name: newrelic-agent
  installed: {{ eq .Environment.Name "production" | toYaml }}
  # snip
- name: myapp
  # snip
```

### Environment Values
Helmfile supports 3 values languages :
- Straight yaml
- Go templates to generate straight yaml
- HCL

Environment Values allows you to inject a set of values specific to the selected environment, into `values.yaml` templates.
Use it to inject common values from the environment to multiple values files, to make your configuration DRY.

Suppose you have three files `helmfile.yaml`, `production.yaml` and `values.yaml.gotmpl`:

`helmfile.yaml`

```yaml
environments:
  production:
    values:
    - production.yaml

---

releases:
- name: myapp
  values:
  - values.yaml.gotmpl
```

`production.yaml`

```yaml
domain: prod.example.com
releaseName: prod
```

`values.yaml.gotmpl`

```yaml
domain: {{ .Values | get "domain" "dev.example.com" }}
```

`helmfile sync` installs `myapp` with the value `domain=dev.example.com`,
whereas `helmfile --environment production sync` installs the app with the value `domain=prod.example.com`.

For even more flexibility, you can now use values declared in the `environments:` section in other parts of your helmfiles:

consider:
`default.yaml`

```yaml
domain: dev.example.com
releaseName: dev
```

```yaml
environments:
  default:
    values:
    - default.yaml
  production:
    values:
    - production.yaml #  bare .yaml file, content will be used verbatim
    - other.yaml.gotmpl  #  template directives with potential side-effects like `exec` and `readFile` will be honoured

---

releases:
- name: myapp-{{ .Values.releaseName }} # release name will be one of `dev` or `prod` depending on selected environment
  values:
  - values.yaml.gotmpl
- name: production-specific-release
  # this release would be installed only if selected environment is `production`
  installed: {{ eq .Values.releaseName "prod" | toYaml }}
  ...
```

#### HCL specifications

Since Helmfile v0.164.0, HCL language is supported for environment values only.
HCL values supports interpolations and sharing values across files

* Only `.hcl` suffixed files will be interpreted as is
* Helmfile supports 2 different blocks: `values` and `locals`
* `values` block is a shared block where all values are accessible everywhere in all loaded files
* `locals` block can't reference external values apart from the ones in the block itself, and where its defined values are only accessible in its local file
* Only values in `values` blocks are made available to the final root `.Values` (e.g : ` values { myvar = "var" }` is accessed through `{{ .Values.myvar }}`)
* There can only be 1 `locals` block per file
* Helmfile hcl `values` are referenced using the `hv` accessor.
* Helmfile hcl `locals` are referenced using the `local` accessor.
* When the same key is defined multiple times across imported `.hcl` files in `values` blocks, values from later files override those from earlier files (last file loaded wins). Map values are merged per key, while list values are replaced as a whole (i.e. not deep-merged). Mixed-types overrides (e.g. bool -> string) are supported (latest value/type wins).
* All cty [standard library functions](`https://pkg.go.dev/github.com/zclconf/go-cty@v1.14.3/cty/function/stdlib`) are available and custom functions could be created in the future

Consider the following example :

```terraform
# values1.hcl
locals {
  hostname = "host1"
}
values {
  domain = "DEV.EXAMPLE.COM"
  hostnameV1 = "${local.hostname}.${lower(hv.domain)}" # "host1.dev.example.com"
}
```
```terraform
# values2.hcl
locals {
  hostname = "host2"
}

values {
  hostnameV2 = "${local.hostname}.${hv.domain}" # "host2.DEV.EXAMPLE.COM"
}
```
#### Note on Environment.Values vs Values

The `{{ .Values.foo }}` syntax is the recommended way of using environment values.

Prior to this [pull request](https://github.com/roboll/helmfile/pull/647), environment values were made available through the `{{ .Environment.Values.foo }}` syntax.
This is still working but is **deprecated** and the new `{{ .Values.foo }}` syntax should be used instead.

You can read more infos about the feature proposal [here](https://github.com/roboll/helmfile/issues/640).

### Environment Secrets

Environment Secrets *(not to be confused with Kubernetes Secrets)* are encrypted versions of `Environment Values`.
You can list any number of `secrets.yaml` files created using `helm secrets` or `sops`, so that
Helmfile could automatically decrypt and merge the secrets into the environment values.

First you must have the [helm-secrets](https://github.com/jkroepke/helm-secrets) plugin installed along with a
`.sops.yaml` file to configure the method of encryption (this can be in the same directory as your helmfile or
in the subdirectory containing your secrets files).

Then suppose you have a secret `foo.bar` defined in `environments/production/secrets.yaml`:

```yaml
foo.bar: "mysupersecretstring"
```

You can then encrypt it with `helm secrets enc environments/production/secrets.yaml`

Then reference that encrypted file in `helmfile.yaml`:

```yaml
environments:
  production:
    secrets:
    - environments/production/secrets.yaml

---

releases:
- name: myapp
  chart: mychart
  values:
  - values.yaml.gotmpl
```

Then the environment secret `foo.bar` can be referenced by the below template expression in your `values.yaml.gotmpl`:

```yaml
{{ .Values.foo.bar }}
```

#### Loading remote Environment secrets files

Since Helmfile v0.149.0, you can use `go-getter`-style URLs to refer to remote secrets files, the same way as in values files:
```yaml
environments:
  staging:
    secrets:
      - git::https://{{ env "GITHUB_PAT" }}@github.com/org/repo.git@/environments/staging.secret.yaml?ref=main
      - http://$HOSTNAME/artifactory/example-repo-local/test.tgz@environments/staging.secret.yaml
  production:
    secrets:
      - git::https://{{ env "GITHUB_PAT" }}@github.com/org/repo.git@/environments/production.secret.yaml?ref=main
      - http://$HOSTNAME/artifactory/example-repo-local/test.tgz@environments/production.secret.yaml
```

### Loading remote Environment values files

Since Helmfile v0.118.8, you can use `go-getter`-style URLs to refer to remote values files:

```yaml
environments:
  cluster-azure-us-west:
    values:
      - git::https://git.company.org/helmfiles/global/azure.yaml?ref=master
      - git::https://git.company.org/helmfiles/global/us-west.yaml?ref=master
      - git::https://gitlab.com/org/repository-name.git@/config/config.test.yaml?ref=main # Public Gilab Repo
  cluster-gcp-europe-west:
    values:
      - git::https://git.company.org/helmfiles/global/gcp.yaml?ref=master
      - git::https://git.company.org/helmfiles/global/europe-west.yaml?ref=master
      - git::https://ci:{{ env "CI_JOB_TOKEN" }}@gitlab.com/org/repository-name.git@/config.dev.yaml?ref={{ env "APP_COMMIT_SHA" }}  # Private Gitlab Repo
  staging:
     values:
      - git::https://{{ env "GITHUB_PAT" }}@github.com/[$GITHUB_ORGorGITHUB_USER]/repository-name.git@/values.dev.yaml?ref=main #Github Private repo
      - http://$HOSTNAME/artifactory/example-repo-local/test.tgz@values.yaml #Artifactory url
---

releases:
  - ...
```

Since Helmfile v0.158.0, support more protocols, such as: s3, https, http
```
values:
  - s3::https://helm-s3-values-example.s3.us-east-2.amazonaws.com/values.yaml
  - s3://helm-s3-values-example/subdir/values.yaml
  - https://john:doe@helm-s3-values-example.s3.us-east-2.amazonaws.com/values.yaml
  - http://helm-s3-values-example.s3.us-east-2.amazonaws.com/values.yaml
```

For more information about the supported protocols see: [go-getter Protocol-Specific Options](https://github.com/hashicorp/go-getter#protocol-specific-options-1).

This is particularly useful when you co-locate helmfiles within your project repo but want to reuse the definitions in a global repo.

### Environment values precedence
With the introduction of HCL, a new value precedence was introduced over environment values.
Here is the order of precedence from least to greatest (the last one overrides all others)
1. `yaml` / `yaml.gotmpl`
2. `hcl`
3. `yaml` secrets

Example:

---

```yaml
# values1.yaml
domain: "dev.example.com"
```

```terraform
# values2.hcl
values {
  domain = "overdev.example.com"
  env = "dev"
  willBeOverridden = "override_me"
}
```

```terraform
# values3.hcl
values {
  env = "local"
}
```

```yaml
# secrets.yml (assuming this one has been encrypted)
willBeOverridden: overridden
```

```
# helmfile.yaml.gotmpl
environments:
  default:
    values:
    - values1.yaml
    - values2.hcl
    - values3.hcl
    secrets:
    - secrets.yml
---
releases:
- name: random-release
  [...]
  values:
    domain: "{{ .Values.domain }}" # == "overdev.example.com"
    env: "{{ .Values.env }}" # == "local"
    willBeOverridden: "{{ .Values.willBeOverridden }}" # == "overridden"
```

### Environment defaults

In addition to `values`, environments support a `defaults` block that provides a separate layer of default values. These are merged **before** `values`, giving `values` higher priority:

```yaml
environments:
  default:
    defaults:
      - cluster: dev
        replicas: 1
    values:
      - replicas: 3
```

The merge order for environment values is:

```
┌─────────────────────────────────────────────────────────────────┐
│  1. Environment defaults (merged first, lowest priority)        │
│  2. Environment values (yaml/yaml.gotmpl)                       │
│  3. Environment values (HCL)                                    │
│  4. Environment secrets (non-HCL, decrypted)                    │
│  5. CLI overrides (--state-values-set, --state-values-file)     │
└─────────────────────────────────────────────────────────────────┘
```

In the example above, `{{ .Values.replicas }}` would be `3` (values overrides defaults) and `{{ .Values.cluster }}` would be `dev` (only defined in defaults).
