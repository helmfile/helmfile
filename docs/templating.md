# Templating

Helmfile uses [Go templates](https://godoc.org/text/template) for templating your helmfile.yaml. While go ships several built-in functions, we have added all of the functions in the [Sprig library](https://godoc.org/github.com/Masterminds/sprig).

We also added the following functions:

* [`env`](templating_funcs.md#env)
* [`requiredEnv`](templating_funcs.md#requiredenv)
* [`exec`](templating_funcs.md#exec)
* [`envExec`](templating_funcs.md#envexec)
* [`readFile`](templating_funcs.md#readfile)
* [`readDir`](templating_funcs.md#readdir)
* [`readDirEntries`](templating_funcs.md#readdirentries)
* [`toYaml`](templating_funcs.md#toyaml)
* [`fromYaml`](templating_funcs.md#fromyaml)
* [`setValueAtPath`](templating_funcs.md#setvalueatpath)
* [`get`](templating_funcs.md#get) (Sprig's original `get` is available as `sprigGet`)
* [`getOrNil`](templating_funcs.md#getornil)
* [`tpl`](templating_funcs.md#tpl)
* [`required`](templating_funcs.md#required)
* [`fetchSecretValue`](templating_funcs.md#fetchsecretvalue)
* [`expandSecretRefs`](templating_funcs.md#expandsecretrefs)
* [`include`](templating_funcs.md#include)

More details on each function can be found at the ["Template Functions" page in our documentation](templating_funcs.md).

## Using environment variables

Environment variables can be used in most places for templating the helmfile. Currently this is supported for `name`, `namespace`, `value` (in set), `values` and `url` (in repositories).

Examples:

```yaml
repositories:
- name: your-private-git-repo-hosted-charts
  url: https://{{ requiredEnv "GITHUB_TOKEN"}}@raw.githubusercontent.com/kmzfs/helm-repo-in-github/master/
```

```yaml
releases:
  - name: {{ requiredEnv "NAME" }}-vault
    namespace: {{ requiredEnv "NAME" }}
    chart: roboll/vault-secret-manager
    values:
      - db:
          username: {{ requiredEnv "DB_USERNAME" }}
          password: {{ requiredEnv "DB_PASSWORD" }}
    set:
      - name: proxy.domain
        value: {{ requiredEnv "PLATFORM_ID" }}.my-domain.com
      - name: proxy.scheme
        value: {{ env "SCHEME" | default "https" }}
```

### Note

If you wish to treat your enviroment variables as strings always, even if they are boolean or numeric values you can use `{{ env "ENV_NAME" | quote }}` or `"{{ env "ENV_NAME" }}"`. These approaches also work with `requiredEnv`.

### Useful internal Helmfile environment variables

Helmfile uses some OS environment variables to override default behaviour:

* `HELMFILE_DISABLE_INSECURE_FEATURES` - disable insecure features, expecting `true` lower case
* `HELMFILE_DISABLE_RUNNER_UNIQUE_ID` - disable unique logging ID, expecting any non-empty value
* `HELMFILE_SKIP_INSECURE_TEMPLATE_FUNCTIONS` - disable insecure template functions, expecting `true` lower case
* `HELMFILE_USE_HELM_STATUS_TO_CHECK_RELEASE_EXISTENCE` - expecting non-empty value to use `helm status` to check release existence, instead of `helm list` which is the default behaviour
* `HELMFILE_EXPERIMENTAL` - enable experimental features, expecting `true` lower case
* `HELMFILE_ENVIRONMENT` - specify [Helmfile environment](environments.md), it has lower priority than CLI argument `--environment`
* `HELMFILE_TEMPDIR` - specify directory to store temporary files
* `HELMFILE_UPGRADE_NOTICE_DISABLED` - expecting any non-empty value to skip the check for the latest version of Helmfile in [helmfile version](cli.md#version)
* `HELMFILE_GO_YAML_V3` - use *go.yaml.in/yaml/v3* instead of *go.yaml.in/yaml/v2*.  It's `false` by default in Helmfile v0.x, and `true` in Helmfile v1.x.
* `HELMFILE_CACHE_HOME` - specify directory to store cached files for remote operations
* `HELMFILE_FILE_PATH` - specify the path to the helmfile.yaml file
* `HELMFILE_INTERACTIVE` - enable interactive mode, expecting `true` lower case. The same as `--interactive` CLI flag
* `HELMFILE_RENDER_YAML` - force helmfile.yaml to be rendered as a Go template regardless of file extension, expecting `true` lower case. Useful for migrating from v0 to v1 without renaming files to `.gotmpl`
* `HELMFILE_AWS_SDK_LOG_LEVEL` - configure AWS SDK logging level for vals library. Valid values: `off` (default, secure, case-insensitive), `minimal`, `standard`, `verbose`, or custom comma-separated values like `request,response`. See issue #2270 for details
* `HELMFILE_VALS_FAIL_ON_MISSING_KEY_IN_MAP` - enable strict mode for vals secret references. When set to `true` (or any value accepted by Go's `strconv.ParseBool` like `TRUE`, `1`), vals will fail when a referenced key does not exist in the secret map. Invalid values will cause an error when vals is initialized (when secret refs are first evaluated). Default is `false` (when unset or empty) for backward compatibility. See issue #1563 for details


## Templates

You can use go's text/template expressions in `helmfile.yaml` and `values.yaml.gotmpl` (templated helm values files). `values.yaml` references will be used verbatim. In other words:

* for value files ending with `.gotmpl`, template expressions will be rendered
* for plain value files (ending in `.yaml`), content will be used as-is

In addition to built-in ones, the following custom template functions are available:

* `readFile` reads the specified local file and generate a golang string
* `readDir` reads the files within provided directory path. (folders are excluded)
* `readDirEntries` Returns a list of [https://pkg.go.dev/os#DirEntry](DirEntry) within provided directory path
* `fromYaml` reads a golang string and generates a map
* `setValueAtPath PATH NEW_VALUE` traverses a golang map, replaces the value at the PATH with NEW_VALUE
* `toYaml` marshals a map into a string
* `get` returns the value of the specified key if present in the `.Values` object, otherwise will return the default value defined in the function

### Values Files Templates

You can reference a template of values file in your `helmfile.yaml` like below:

```yaml
releases:
- name: myapp
  chart: mychart
  values:
  - values.yaml.gotmpl
```

Every values file whose file extension is `.gotmpl` is considered as a template file.

Suppose `values.yaml.gotmpl` was something like:

```yaml
{{ readFile "values.yaml" | fromYaml | setValueAtPath "foo.bar" "FOO_BAR" | toYaml }}
```

And `values.yaml` was:

```yaml
foo:
  bar: ""
```

The resulting, temporary values.yaml that is generated from `values.yaml.gotmpl` would become:

```yaml
foo:
  # Notice `setValueAtPath "foo.bar" "FOO_BAR"` in the template above
  bar: FOO_BAR
```

## Refactoring `helmfile.yaml` with values files templates

One of expected use-cases of values files templates is to keep `helmfile.yaml` small and concise.

See the example `helmfile.yaml` below:

```yaml
releases:
  - name: {{ requiredEnv "NAME" }}-vault
    namespace: {{ requiredEnv "NAME" }}
    chart: roboll/vault-secret-manager
    values:
      - db:
          username: {{ requiredEnv "DB_USERNAME" }}
          password: {{ requiredEnv "DB_PASSWORD" }}
    set:
      - name: proxy.domain
        value: {{ requiredEnv "PLATFORM_ID" }}.my-domain.com
      - name: proxy.scheme
        value: {{ env "SCHEME" | default "https" }}
```

The `values` and `set` sections of the config file can be separated out into a template:

`helmfile.yaml`:

```yaml
releases:
  - name: {{ requiredEnv "NAME" }}-vault
    namespace: {{ requiredEnv "NAME" }}
    chart: roboll/vault-secret-manager
    values:
    - values.yaml.gotmpl
```

`values.yaml.gotmpl`:

```yaml
db:
  username: {{ requiredEnv "DB_USERNAME" }}
  password: {{ requiredEnv "DB_PASSWORD" }}
proxy:
  domain: {{ requiredEnv "PLATFORM_ID" }}.my-domain.com
  scheme: {{ env "SCHEME" | default "https" }}
```

## Importing values from any source

The `exec` template function that is available in `values.yaml.gotmpl` is useful for importing values from any source
that is accessible by running a command:

A usual usage of `exec` would look like this:

```yaml
mysetting: |
{{ exec "./mycmd" (list "arg1" "arg2" "--flag1") | indent 2 }}
```

Or even with a pipeline:

```yaml
mysetting: |
{{ yourinput | exec "./mycmd-consume-stdin" (list "arg1" "arg2") | indent 2 }}
```

The possibility is endless. Try importing values from your golang app, bash script, jsonnet, or anything!

Then `envExec` same as `exec`, but it can receive a dict as the envs.

A usual usage of `envExec` would look like this:

```yaml
mysetting: |
{{ envExec (dict "envkey" "envValue") "./mycmd" (list "arg1" "arg2" "--flag1") | indent 2 }}
```

## Using .env files

Helmfile itself doesn't have an ability to load .env files. But you can write some bash script to achieve the goal:

```console
set -a; . .env; set +a; helmfile sync
```

Please see #203 for more context.

## Running Helmfile interactively

`helmfile --interactive [apply|destroy|delete|sync]` requests confirmation from you before actually modifying your cluster.

Use it when you're running `helmfile` manually on your local machine or a kind of secure administrative hosts.

For your local use-case, aliasing it like `alias hi='helmfile --interactive'` would be convenient.

Another way to use it is to set the environment variable `HELMFILE_INTERACTIVE=true` to enable the interactive mode by default.
Anything other than `true` will disable the interactive mode. The precedence has the `--interactive` flag.

## Running Helmfile without an Internet connection

Once you download all required charts into your machine, you can run `helmfile sync --skip-deps` to deploy your apps.
With the `--skip-deps` option, you can skip running "helm repo update" and "helm dependency build".

## `bash` and `zsh` completion

helmfile completion --help
