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
