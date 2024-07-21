## Advanced Features

- [Import Configuration Parameters into Helmfile](#import-configuration-parameters-into-helmfile)
- [Deploy Kustomization with Helmfile](#deploy-kustomizations-with-helmfile)
- [Adhoc Kustomization of Helm Charts](#adhoc-kustomization-of-helm-charts)
- [Adding dependencies without forking the chart](#adding-dependencies-without-forking-the-chart)

### Import Configuration Parameters into Helmfile

Helmfile integrates [vals]() to import configuration parameters from following backends:

- AWS SSM Parameter Store
- AWS SecretsManager
- Vault
- SOPS

See [Vals "Supported Backends"](https://github.com/helmfile/vals#supported-backends) for the full list of available backends.

This feature was implemented in https://github.com/roboll/helmfile/pull/906.
If you're curious about how it's designed and how it works, please review the pull request.

### Deploy Kustomizations with Helmfile

You can deploy [kustomize](https://github.com/kubernetes-sigs/kustomize) "kustomization"s with Helmfile.

Most Kustomize operations are usually done with `kustomize edit` and can be done declaratively via Helm `values.yaml` files.

Under the hood, Helmfile transforms the kustomization into a local chart in a temporary directory so that it can be `helm upgrade --install`ed.

The transformation is done by generating (1)a temporary kustomization from various options and (2)temporary chart from the temporary kustomization.

An example pseudo code for the transformation logic can be written as:

```console
$ TMPCHART=/tmp/sometmpdir
$ mkdir -p ${TMPCHART}/templates
$ somehow_generate_chart_yaml ${TMPCHART}/Chart.yaml

$ TMPKUSTOMIZATION=/tmp/sometmpdir2
$ somehow_generate_temp_kustomization_yaml ${TMPKUSTOMIZATION}/kustomization.yaml
$ kustomize build ${TMPKUSTOMIZATION}/kustomization.yaml > ${TMPCHART}/templates/all.yaml
```

Let's say you have a `helmfile.yaml` that looks like the below:

```yaml
releases:
- name: myapp
  chart: mykustomization
  values:
  - values.yaml
```

Helmfile firstly generates a temporary `kustomization.yaml` that looks like:

```yaml
bases:
- $(ABS_PATH_TO_HELMFILE_YAML}/mykustomization
```

Followed by the below steps:

- Running `kustomize edit set image $IMAGE` for every `$IMAGE` generated from your values.yaml
- Running `kustomize edit set nameprefix $NAMEPREFIX` with the nameprefix specified in your values.yaml
- Running `kustomize edit set namesuffix $NAMESUFFIX` with the namesuffix specified in your values.yaml
- Running `kustomize edit set namespace $NS` with the namespace specified in your values.yaml

A `values.yaml` file for kustomization would look like the below:

```yaml
images:
# kustomize edit set image mysql=eu.gcr.io/my-project/mysql@canary
- name: mysql
  newName: eu.gcr.io/my-project/mysql
  newTag: canary
# kustomize edit set image myapp=my-registry/my-app@sha256:24a0c4b4a4c0eb97a1aabb8e29f18e917d05abfe1b7a7c07857230879ce7d3d3
- name: myapp
  digest: sha256:24a0c4b4a4c0eb97a1aabb8e29f18e917d05abfe1b7a7c07857230879ce7d3d3
  newName: my-registry/my-app

# kustomize edit set nameprefix foo-
namePrefix: foo-

# kustomize edit set namesuffix -bar
nameSuffix: -bar

# kustomize edit set namespace myapp
namespace: myapp
```

At this point, Helmfile can generate a complete kustomization from the base kustomization you specified in `releases[].chart` of your helmfile.yaml and `values.yaml`,
which can be included in the temporary chart.

After all, Helmfile just installs the temporary chart like standard charts, which allows you to manage everything with Helmfile regardless of each app is declared using a Helm chart or a kustomization.

Please also see [test/advanced/helmfile.yaml](https://github.com/helmfile/helmfile/tree/master/test/advanced/helmfile.yaml) for an example of kustomization support and more.

### Adhoc Kustomization of Helm charts

With Helmfile's integration with Kustomize, not only deploying Kustomization as a Helm chart, you can kustomize charts before installation.

.. Hint:: The following fields can also specify files, in the same manner as
   the `values` field.

Currently, Helmfile allows you to set the following fields for kustomizing the chart:

- [`releases[].strategicMergePatches`](#strategicmergepatches)
- `releases[].jsonPatches`
- [`releases[].transformers`](#transformers)

#### `strategicMergePatches`

You can add/update any Kubernetes resource field rendered from a Helm chart by specifying `releases[].strategicMergePatches`:

```
repositories:
- name: incubator
  url: https://charts.helm.sh/incubator

releases:
- name: raw1
  chart: incubator/raw
  values:
  - resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: raw1
        namespace: default
      data:
        foo: FOO
  strategicMergePatches:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: raw1
        namespace: default
      data:
        bar: BAR
```

Running `helmfile template` on the above example results in a ConfigMap called `raw` whose `data` is:

```yaml
foo: FOO
bar: BAR
```

Please note that the second `data` field `bar` is coming from the strategic-merge patch defined in the above helmfile.yaml.

There's also `releases[].jsonPatches` that works similarly to `strategicMergePatches` but has additional capability to remove fields.

Please also see [test/advanced/helmfile.yaml](https://github.com/helmfile/helmfile/tree/master/test/advanced/helmfile.yaml) for an example of patching support and more.

#### `transformers`

You can set `transformers` to apply [Kustomize's transformers](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/configureBuiltinPlugin.md#configuring-the-builtin-plugins-instead).

Each item can be a path to a YAML or Go template file, or an embedded transformer declaration as a YAML hash.

It's often used to add common labels and annotations to your resources.

In the below example, we add common annotations and labels every resource rendered from the `aws-load-balancer-controller` chart:

```yaml
releases:
- name: "aws-load-balancer-controller"
  namespace: "kube-system"
  forceNamespace: "kube-system"
  chart: "center/aws/aws-load-balancer-controller"
  transformers:
  - apiVersion: builtin
    kind: AnnotationsTransformer
    metadata:
      name: notImportantHere
    annotations:
      area: 51
      greeting: take me to your leader
    fieldSpecs:
    - path: metadata/annotations
      create: true
  - apiVersion: builtin
    kind: LabelTransformer
    metadata:
      name: notImportantHere
    labels:
      foo: bar
    fieldSpecs:
    - path: metadata/labels
      create: true
```

As explained earlier, `transformers` can be not only a list of embedded transformers, but also YAML or Go template files, or a mix of those three kinds.

```yaml
transformers:
# Embedded transformer
- apiVersion: builtin
  kind: AnnotationsTransformer
  metadata:
    name: notImportantHere
  annotations:
    area: 51
    greeting: take me to your leader
  fieldSpecs:
  - path: metadata/annotations
    create: true
# YAML file
- path/to/transformer.yaml
# Go template
# The same set of template parameters as release values files templates is available.
- path/to/transformer.yaml.gotmpl
```

Please see https://github.com/kubernetes-sigs/kustomize/blob/master/examples/configureBuiltinPlugin.md#configuring-the-builtin-plugins-instead for more information on how to declare transformers.

### Adding dependencies without forking the chart

With Helmfile, you can add chart dependencies to a Helm chart without forking it.

An example `helmfile.yaml` that adds a `stable/envoy` dependency to the release `foo` looks like the below:

```yaml
repositories:
- name: stable
  url: https://charts.helm.sh/stable

releases:
- name: foo
  chart: ./path/to/foo
  dependencies:
  - chart: stable/envoy
    version: 1.5
```

When Helmfile encounters `releases[].dependencies`, it creates another temporary chart from `./path/to/foo` and adds the following `dependencies` to the `Chart.yaml`, so that you don't need to fork the chart.

```yaml
dependencies:
- name: envoy
  repo: https://charts.helm.sh/stable
  condition: envoy.enabled
```

A Helm chart can have two or more dependencies for the same chart with different `alias`es. To give your dependency an `alias`, define it like you would do in a standard `Chart.yaml`:

```yaml
repositories:
- name: stable
  url: https://charts.helm.sh/stable

releases:
- name: foo
  chart: ./path/to/foo
  dependencies:
  - chart: stable/envoy
    version: 1.5
    alias: bar
  - chart: stable/envoy
    version: 1.5
    alias: baz
```

which will tweaks the temporary chart's `Chart.yaml` to have:


```yaml
dependencies:
- alias: bar
  name: envoy
  repo: https://charts.helm.sh/stable
  condition: bar.enabled
- alias: baz
  name: envoy
  repo: https://charts.helm.sh/stable
  condition: baz.enabled
```

Please see #649 for more context around this feature.

After the support for adhoc dependency to local chart (#1765),
you can even write local file paths relative to `helmfile.yaml` in `chart`:

```yaml
releases:
- name: foo
  chart: ./path/to/foo
  dependencies:
  - chart: ./path/to/bar
```

Internally, Helmfile creates another temporary chart from the local chart `./path/to/foo`, and modifies the chart's `Chart.yaml` dependencies to look like:

```yaml
dependencies:
- alias: bar
  name: bar
  repo: file:///abs/path/to/bar
  condition: bar.enabled
```

Please read https://github.com/roboll/helmfile/issues/1762#issuecomment-816341251 for more details.

#### OCI chart dependencies

With Helmfile version v0.146.0 or later, you can add OCI chart to chart dependencies.

An example `helmfile.yaml` that adds a OCI chart dependency to the release `foo` looks like the below:

```yaml
releases:
- name: foo
  chart: ./path/to/foo
  dependencies:
  - chart: oci://my-oci-registry/helm-repo/envoy
    version: 1.5
```

### Lockfile per environment

In some cases it can be handy for CI/CD pipelines to be able to roll out updates gradually for environments, such as staging and production while using the same
set of charts. This can be achieved by using `lockFilePath` in combination with environments, such as:

```yaml
environments:
  staging:
  production

---
lockFilePath: .helmfile.{{ .Environment.Name}}.lock

releases:
- name: myapp
  chart: charts/myapp
```
