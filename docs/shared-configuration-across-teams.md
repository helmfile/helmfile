# Shared Configuration Across Teams

Assume you have two or more teams, each work for a different internal or external service, like:

- Product 1
- Product 2
- Observability

The simplest `helmfile.yaml` that declares the whole cluster that is composed of the three services would look like the below:

```yaml
releases:
- name: product1-api
  chart: product1-charts/api
  # snip
- name: product1-web
  chart: product1-charts/web
  # snip
- name: product2-api
  chart: saas-charts/api
  # snip
- name: product2-web
  chart: product2-charts/web
  # snip
- name: observability-prometheus-operator
  chart: stable/prometheus-operator
  # snip
- name: observability-process-exporter
  chart: stable/prometheus-operator
  # snip
```

This works, but what if you wanted to a separate cluster per service to achieve a smaller blast radius?

Let's start by creating a `helmfile.yaml` for each service.

`product1/helmfile.yaml`:

```yaml
releases:
- name: product1-api
  chart: product1-charts/api
  # snip
- name: product1-web
  chart: product1-charts/web
  # snip
- name: observability-prometheus-operator
  chart: stable/prometheus-operator
  # snip
- name: observability-process-exporter
  chart: stable/prometheus-operator
  # snip
```

`product2/helmfile.yaml`:

```yaml
releases:
- name: product2-api
  chart: product2-charts/api
  # snip
- name: product2-web
  chart: product2-charts/web
  # snip
- name: observability-prometheus-operator
  chart: stable/prometheus-operator
  # snip
- name: observability-process-exporter
  chart: stable/prometheus-operator
  # snip
```

You will (of course!) notice this isn't DRY.

To remove the duplication of observability stack between the two helmfiles, create a "sub-helmfile" for the observability stack.

`observability/helmfile.yaml`:

```yaml
- name: observability-prometheus-operator
  chart: stable/prometheus-operator
  # snip
- name: observability-process-exporter
  chart: stable/prometheus-operator
  # snip
```

As you might have imagined, the observability helmfile can be reused from the two product helmfiles by declaring `helmfiles`.

`product1/helmfile.yaml`:

```yaml
helmfiles:
- ../observability/helmfile.yaml

releases:
- name: product1-api
  chart: product1-charts/api
  # snip
- name: product1-web
  chart: product1-charts/web
  # snip
```

`product2/helmfile.yaml`:

```yaml
helmfiles:
- ../observability/helmfile.yaml

releases:
- name: product2-api
  chart: product2-charts/api
  # snip
- name: product2-web
  chart: product2-charts/web
  # snip
```

## Using sub-helmfile as a template

You can go even further by generalizing the product related releases as a pair of `api` and `web`:

`shared/helmfile.yaml`:

```yaml
releases:
- name: product{{ env "PRODUCT_ID" }}-api
  chart: product{{ env "PRODUCT_ID" }}-charts/api
  # snip
- name: product{{ env "PRODUCT_ID" }}-web
  chart: product{{ env "PRODUCT_ID" }}-charts/web
  # snip
```

Then you only need one single product helmfile


`product/helmfile.yaml`:

```yaml
helmfiles:
- ../observability/helmfile.yaml
- ../shared/helmfile.yaml
```

Now that we use the environment variable `PRODUCT_ID` to as the parameters of release names, you need to set it before running `helmfile`, so that it produces the differently named releases per product:

```console
$ PRODUCT_ID=1 helmfile -f product/helmfile.yaml apply
$ PRODUCT_ID=2 helmfile -f product/helmfile.yaml apply
```

## Inheriting parent configuration with `inherits:`

By default a sub-helmfile is independent: it does **not** see the `repositories:`,
`helmDefaults:`, `environments:`, etc. declared in the parent that includes it. To
share configuration you either extract it into a separate file referenced via
[`bases:`](writing-helmfile.md#layering-state-files) from each sub-helmfile, or — more concisely — let each sub-helmfile
opt into inheriting specific categories from its parent with `inherits:`.

```yaml
# parent helmfile.yaml
repositories:
- name: release-charts
  url: registry.example.com/release/helm-charts
  oci: true

helmfiles:
- path: myapp.yaml
  inherits:
  - repositories
  - environments
```

`myapp.yaml` now sees the `release-charts` repository and the parent's resolved
environment values, without having to re-declare them.

### Allowed values

`inherits:` accepts a list. Each entry must be one of:

| Key | What is inherited |
|-----|-------------------|
| `repositories` | Parent repository definitions (appended; child entry wins on name conflict). |
| `helmDefaults` | Parent helm defaults; parent fills the child's unset sub-fields, child's set sub-fields win. |
| `commonLabels` | Parent common labels; child's keys win on conflict. |
| `apiVersions` | Parent API versions (appended and de-duplicated). |
| `kubeVersion` | Parent kube version, only when the child left it empty. |
| `templates` | Parent templates; child's template wins on name conflict. |
| `environments` | The parent's **resolved** environment values (see note below). |

### Precedence

**Child wins, parent fills gaps** — consistent with `bases:`. Slices like
`repositories` accumulate (parent first); maps (`commonLabels`, `templates`) are
unioned with the child's keys winning; `helmDefaults` is deep-merged so the child
overrides individual sub-fields it sets.

### Note on `environments`

`environments` inherits the parent's **already-resolved** values (including CLI
overrides and decrypted secrets), not the `environments:` declaration block. This
avoids ambiguity around the directory that relative values files resolve against
(the parent resolves them once). The child's own `environments:` block, if any,
still takes precedence per key.

### Transitive inheritance

Inheritance is opt-in at **every** level but the values propagate. If a parent
inherits `repositories` and the child in turn has its own `helmfiles:` entries
declaring `inherits: [repositories]`, the grandchild receives the accumulated set.

### `inherits:` vs `bases:`

| | `bases:` | `inherits:` |
|---|---|---|
| Model | Pull — each file lists files to merge into itself | Push — the parent pushes its config to the child |
| Where the shared config lives | Must live in a separate file | Can live inline in the parent |
| Repetition | `bases:` must be repeated in every sub-helmfile | Declared once, per sub-helmfile entry |

They are complementary: use `bases:` for cross-team reusable building blocks
(environment defaults, shared helm defaults), and `inherits:` to let a sub-helmfile
consume the parent's inline configuration without duplication.

### Footgun warning

If a sub-helmfile references a repository that the parent declares but the child
does not (and `repositories` is not inherited), helmfile prints a warning suggesting
`inherits: [repositories]` instead of failing later with a confusing
`repo not found` (see [#1495](https://github.com/helmfile/helmfile/issues/1495)).

