# Releases & DAG

## DAG-aware installation/deletion ordering with `needs`

`needs` controls the order of the installation/deletion of the release:

```yaml
releases:
- name: somerelease
  needs:
  - [[KUBECONTEXT/]NAMESPACE/]anotherelease
```

Be aware that you have to specify the kubecontext and namespace name if you configured one for the release(s).

All the releases listed under `needs` are installed before(or deleted after) the release itself.

For the following example, `helmfile [sync|apply]` installs releases in this order:

1. logging
2. servicemesh
3. myapp1 and myapp2

```yaml
  - name: myapp1
    chart: charts/myapp
    needs:
    - servicemesh
    - logging
  - name: myapp2
    chart: charts/myapp
    needs:
    - servicemesh
    - logging
  - name: servicemesh
    chart: charts/istio
    needs:
    - logging
  - name: logging
    chart: charts/fluentd
```

Note that all the releases in a same group is installed concurrently. That is, myapp1 and myapp2 are installed concurrently.

On `helmfile [delete|destroy]`, deletions happen in the reverse order.

That is, `myapp1` and `myapp2` are deleted first, then `servicemesh`, and finally `logging`.

### Selectors and `needs`

When using selectors/labels, `needs` are ignored by default. This behaviour can be overruled with a few parameters:

| Parameter | default | Description |
|---|---|---|
| `--skip-needs` | `true` | `needs` are ignored (default behavior).  |
| `--include-needs` | `false` | The direct `needs` of the selected release(s) will be included. |
| `--include-transitive-needs` | `false` | The direct and transitive `needs` of the selected release(s) will be included. |

Let's look at an example to illustrate how the different parameters work:

```yaml
releases:
- name: serviceA
  chart: my/chart
  needs:
  - serviceB
- name: serviceB
  chart: your/chart
  needs:
  - serviceC
- name: serviceC
  chart: her/chart
- name: serviceD
  chart: his/chart
```

| Command | Included Releases Order | Explanation |
|---|---|---|
| `helmfile -l name=serviceA sync` | - `serviceA` | By default no needs are included. |
| `helmfile -l name=serviceA sync --include-needs` | - `serviceB`<br>- `serviceA` | `serviceB` is now part of the release as it is a direct need of `serviceA`.  |
| `helmfile -l name=serviceA sync --include-transitive-needs` | - `serviceC`<br>- `serviceB`<br>- `serviceA` | `serviceC` is now also part of the release as it is a direct need of `serviceB` and therefore a transitive need of `serviceA`.  |

Note that `--include-transitive-needs` will override any potential exclusions done by selectors or conditions. So even if you explicitly exclude a release via a selector it will still be part of the deployment in case it is a direct or transitive need of any of the specified releases.

## Separating helmfile.yaml into multiple independent files

Once your `helmfile.yaml` got to contain too many releases,
split it into multiple yaml files.

Recommended granularity of helmfile.yaml files is "per microservice" or "per team".
And there are two ways to organize your files.

* Single directory
* Glob patterns

### Single directory

`helmfile -f path/to/directory` loads and runs all the yaml files under the specified directory, each file as an independent helmfile.yaml.
The default helmfile directory is `helmfile.d`, that is,
in case helmfile is unable to locate `helmfile.yaml`, it tries to locate `helmfile.d/*.yaml`.

By default, multiple files in `helmfile.d` are processed in **parallel** for better performance. If you need files to be processed **sequentially in alphabetical order** (e.g., for dependency ordering where databases must be deployed before applications), use the `--sequential-helmfiles` flag.

For example, you can use a `<two digit number>-<microservice>.yaml` naming convention to control the sync order when using `--sequential-helmfiles`:

* `helmfile.d`/
  * `00-database.yaml`
  * `01-backend.yaml`
  * `02-frontend.yaml`

```bash
# Process files sequentially in alphabetical order
helmfile --sequential-helmfiles sync
```

> **Note:** When processing multiple helmfile.d files, both parallel and sequential modes resolve paths without changing the process working directory, so relative environment variables like `KUBECONFIG` work correctly.

### Glob patterns

In case you want more control over how multiple `helmfile.yaml` files are organized, use `helmfiles:` configuration key in the `helmfile.yaml`:

Suppose you have multiple microservices organized in a Git repository that looks like:

* `myteam/` (sometimes it is equivalent to a k8s ns, that is `kube-system` for `clusterops` team)
    * `apps/`
        * `filebeat/`
            * `helmfile.yaml` (no `charts/` exists because it depends on the stable/filebeat chart hosted on the official helm charts repository)
            * `README.md` (each app managed by my team has a dedicated README maintained by the owners of the app)
        * `metricbeat/`
            * `helmfile.yaml`
            * `README.md`
        * `elastalert-operator/`
            * `helmfile.yaml`
            * `README.md`
            * `charts/`
                * `elastalert-operator/`
                    * `<the content of the local helm chart>`

The benefits of this structure is that you can run `git diff` to locate in which directory=microservice a git commit has changes.
It allows your CI system to run a workflow for the changed microservice only.

A downside of this is that you don't have an obvious way to sync all microservices at once. That is, you have to run:

```bash
for d in apps/*; do helmfile -f $d diff; if [ $? -eq 2 ]; then helmfile -f $d sync; fi; done
```

At this point, you'll start writing a `Makefile` under `myteam/` so that `make sync-all` will do the job.

It does work, but you can rely on the Helmfile feature instead.

Put `myteam/helmfile.yaml` that looks like:

```yaml
helmfiles:
- apps/*/helmfile.yaml
```

So that you can get rid of the `Makefile` and the bash snippet.
Just run `helmfile sync` inside `myteam/`, and you are done.

All the files are sorted alphabetically per group = array item inside `helmfiles:`, so that you have granular control over ordering, too.

#### selectors

When composing helmfiles you can use selectors from the command line as well as explicit selectors inside the parent helmfile to filter the releases to be used.

```yaml
helmfiles:
- apps/*/helmfile.yaml
- path: apps/a-helmfile.yaml
  selectors:          # list of selectors
  - name=prometheus
  - tier=frontend
- path: apps/b-helmfile.yaml # no selector, so all releases are used
  selectors: []
- path: apps/c-helmfile.yaml # parent selector to be used or cli selector for the initial helmfile
  selectorsInherited: true
```

* When a subhelmfile has explicit `selectors`, those selectors determine which releases from that subhelmfile are considered; parent and CLI selectors are not combined with them for release filtering.
* When CLI selectors are provided (e.g. `helmfile -l name=b sync`) and a subhelmfile has explicit selectors that are provably incompatible with them (same key, different value), that subhelmfile may be **skipped entirely** without loading or rendering it. For example, with `-l name=b`, a subhelmfile with `selectors: [name=a]` will be skipped since no release could match both. This optimization does not apply when `selectorsInherited: true` is set or when no CLI selectors are provided. Use `--debug` to see log messages about skipped subhelmfiles.
* When not selector is specified there are 2 modes for the selector inheritance because we would like to change the current inheritance behavior (see [issue #344](https://github.com/roboll/helmfile/issues/344)  ).
  * Legacy mode, sub-helmfiles without selectors inherit selectors from their parent helmfile. The initial helmfiles inherit from the command line selectors.
  * explicit mode, sub-helmfile without selectors do not inherit from their parent or the CLI selector. If you want them to inherit from their parent selector then use `selectorsInherited: true`. To enable this explicit mode you need to set the following environment variable `HELMFILE_EXPERIMENTAL=explicit-selector-inheritance` (see [experimental](experimental-features.md)).
* Using `selector: []` will select all releases regardless of the parent selector or cli for the initial helmfile
* using `selectorsInherited: true` make the sub-helmfile selects releases with the parent selector or the cli for the initial helmfile. You cannot specify an explicit selector while using `selectorsInherited: true`

