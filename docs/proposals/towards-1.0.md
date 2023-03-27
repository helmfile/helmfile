# Towards Helmfile 1.0

I'd like to make 3 breaking changes to Helmfile and mark it as 1.0, so that we can better communicate with the current and future Helmfile users about our stance on maintaining Helmfile.

Note that every breaking change should have an easy alternative way to achieve the same goal achieved using the removed functionality.

## The changes in 1.0

1. [Forbid the use of `environments` and `releases` within a single helmfile.yaml.gotmpl part](#forbid-the-use-of-environments-and-releases-within-a-single-helmfileyamlgotmpl-part)
2. [Force `.gotmpl` file extension for `helmfile.yaml` in case you want helmfile to render it as a go template before yaml parsing.](#force-gotmpl-file-extension-for-helmfileyaml-in-case-you-want-helmfile-to-render-it-as-a-go-template-before-yaml-parsing)
3. [Remove the `--args` flag from the `helmfile` command](#remove-the---args-flag-from-the-helmfile-command)
4. [Remove `HELMFILE_SKIP_INSECURE_TEMPLATE_FUNCTIONS` in favor of `HELMFILE_DISABLE_INSECURE_FEATURES`](#remove-helmfile_skip_insecure_template_functions-in-favor-of-helmfile_disable_insecure_features)
5. [The long deprecated `charts.yaml` has been finally removed](#the-long-deprecated-chartsyaml-has-been-finally-removed)

### Forbid the use of `environments` and `releases` within a single helmfile.yaml.gotmpl part

  - Helmfile currently relies on a hack called "double rendering" which no one understands correctly (I suppose) to solve the chicken-and-egg problem of rendering the helmfile template(which requires helmfile to parse `environments` as yaml first) and parsing the rendered helmfile as yaml(which requires helmfile to render the template first).
  - By forcing (or print a big warning) the user to do separate helmfile parts for `environments` and `releases`, it's very unlikely Helmfile needs double-rendering at all.
  - After this change, every helmfile.yaml written this way:

        environments:
          default:
            values:
            - foo: bar
        releases:
        - name: myapp
          chart: charts/myapp
          values:
          - {{ .Values | toYaml | nindent 4 }}

    must be rewritten like:

        environments:
          default:
            values:
            - foo: bar
        ---
        releases:
        - name: myapp
          chart: charts/myapp
          values:
          - {{ .Values | toYaml | nindent 4 }}

    It might not be a deal breaker as you already had to separate helmfile parts when you need to generate `environments` dynamically:

        environments:
          default:
            values:
            - foo: bar
        ---
        environments:
          default:
            values:
            - {{ .Values | toYaml | nindent 6}}}
            - bar: baz
        ---
        releases:
        - name: myapp
          chart: charts/myapp
          values:
          - {{ .Values | toYaml | nindent 4 }}

    If you're already using any helmfile.yaml files that are written in the first style, do start using `---` today! It will probably reveal and fix unintended template evaluations. If you start using `---` today, you won't need to do anything after Helmfile 1.0.

### Force `.gotmpl` file extension for `helmfile.yaml` in case you want helmfile to render it as a go template before yaml parsing.

  - As the primary maintainer of the project, I'm tired of explaining why Helmfile renders go template expressions embedded in a yaml comment. [The latest example of it](https://github.com/helmfile/helmfile/issues/127).
  - When we first introduced helmfile the ability to render helmfile.yaml as a go template, I did propose it to require `.gotmpl` file extension to enable the feature. But none of active helmfile users and contributors at that time agreed with it (although I didn't have a very strong opinion on the matter either), so we enabled it without the extension. I consider it as a tech debt now.

### Remove the `--args` flag from the `helmfile` command

  - It has been provided as-is, and never intended to work reliably because it's fundamentally flawed because we have no way to specify which args to be passed to which `helm` commands being called by which `helmfile` command.
  - However, I periodically see a new user finds it's not working and reporting it as a bug([the most recent example](https://github.com/roboll/helmfile/issues/2034#issuecomment-1147059088)). It's not a fault of the user. It's our fault that left this broken feature.
  - Every use-case previsouly covered (by any chance) with `--args` should be covered in new `helmfile.yaml` fields or flags.

### Remove `HELMFILE_SKIP_INSECURE_TEMPLATE_FUNCTIONS` in favor of `HELMFILE_DISABLE_INSECURE_FEATURES`

  - This option didn't make much sense in practical. Generally, you'd want to disable all the insecure features altogether to make your deployment secure, or not disable any features. Disabling all is already possible via `HELMFILE_DISABLE_INSECURE_FEATURES `. In addition, `HELMFILE_SKIP_INSECURE_TEMPLATE_FUNCTIONS` literally made every insecure template function to silently skipped without any error or warning, which made debugging unnecessarily hard when the user accidentally used an insecure function.
  - See https://github.com/helmfile/helmfile/pull/564 for more context.

### The long deprecated `charts.yaml` has been finally removed

Helmfile used to load `helmfile.yaml` or `charts.yaml` when you omitted the `-f` flag. `charts.yaml` has been deprecated for a long time but never been removed. We take v1 as a chance to finally remove it.

## After 1.0

We won't add any backward-incompatible changes while in 1.x, as long as it's inevitable to fix unseen important bug(s).

We also quit saying [Helmfile is in its early days](https://github.com/helmfile/helmfile#status) in your README as... it's just untrue today.
