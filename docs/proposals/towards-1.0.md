# Towards Helmfile 1.0

I'd like to make 3 breaking changes to Helmfile and mark it as 1.0, so that we can better communicate with the current and future Helmfile users about our stance on maintaining Helmfile.

Note that every breaking change should have an easy alternative way to achieve the same goal achieved using the removed functionality.

## Backward compatibility

v1 is backward-compatible with v0.x, except for the following breaking changes.

Each breaking change has an easy alternative way to achieve the same goal achieved using the removed functionality.

We also provide the alternative way in the latest v0.x release before v1.0. That way you can start using the alternative way today and be ready for v1.0. Note that in v0.x, some of those alternative ways are enabled only when `HELMFILE_V1MODE=true` is set.

> Context:
>
> Even though Helmfile had been used in production environments [across multiple organizations](USERS.md), it had been considered to be in its early stage of development, hence versioned 0.x.
>
> Helmfile complies to Semantic Versioning 2.0.0 in which v0.x means that there could be backward-incompatible changes for every release. However, Helmfile has been very conservative about breaking changes, and we had no breaking change for a year or so before start thinking about v1.
>
> That said, you can expect Helmfile v1 to be backward-compatible as much as it was in v0.x.

## The changes in 1.0

1. [Forbid the use of `environments` and `releases` within a single helmfile.yaml.gotmpl part](#forbid-the-use-of-environments-and-releases-within-a-single-helmfileyamlgotmpl-part)
2. [Force `.gotmpl` file extension for `helmfile.yaml` in case you want helmfile to render it as a go template before yaml parsing.](#force-gotmpl-file-extension-for-helmfileyaml-in-case-you-want-helmfile-to-render-it-as-a-go-template-before-yaml-parsing)
3. [Remove the `--args` flag from the `helmfile` command](#remove-the---args-flag-from-the-helmfile-command)
4. [Remove `HELMFILE_SKIP_INSECURE_TEMPLATE_FUNCTIONS` in favor of `HELMFILE_DISABLE_INSECURE_FEATURES`](#remove-helmfile_skip_insecure_template_functions-in-favor-of-helmfile_disable_insecure_features)
5. [The long deprecated `charts.yaml` has been finally removed](#the-long-deprecated-chartsyaml-has-been-finally-removed)
6. [List experimental features](#list-experimental-features)
7. [Remove charts and delete sub-commands](#remove-charts-and-delete-subcommands)

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

  - This option didn't make much sense in practice. Generally, you'd want to disable all the insecure features altogether to make your deployment secure, or not disable any features. Disabling all is already possible via `HELMFILE_DISABLE_INSECURE_FEATURES `. In addition, `HELMFILE_SKIP_INSECURE_TEMPLATE_FUNCTIONS` literally made every insecure template function to silently skipped without any error or warning, which made debugging unnecessarily hard when the user accidentally used an insecure function.
  - See https://github.com/helmfile/helmfile/pull/564 for more context.

### The long deprecated `charts.yaml` has been finally removed

Helmfile used to load `helmfile.yaml` or `charts.yaml` when you omitted the `-f` flag. `charts.yaml` has been deprecated for a long time but never been removed. We take v1 as a chance to finally remove it.

### List experimental features

We have some experimental features that are not stable yet. We should list them in a list and mark them as experimental.

In Helmfile v1.x, all features should be backward-compatible within v1.x as we follow semver. We can't fix features in a backward incompatible way by default. To do so, we need a list of experimental features and say "anything in the experimentals can be modified backward-incompatible ways", and include features that are consireded experimental into the list beforehand.

Why now?

In Helmfile v0.x, all features considered experimental as we follow semver. However, we "ended up" preserving backward-compatibility within v0 and between v0 and v1 "by chance". This doesn't mean anything
introduced in v0 is stable. For example, we might have some features implemented in a very later stage of v0 that are not stable yet. We should mark them as experimental, or we can't fix them in a backward-incompatible way in v1.x. That's why we need to list experimental features now.

### remove-charts-and-delete-subcommands
Now we remove `helmfile charts` and `helmfile delete` subcommands. you can use `helmfile destroy` and `helmfile sync` instead.

## After 1.0

We won't add any backward-incompatible changes while in 1.x, as long as it's inevitable to fix unseen important bug(s).

We also quit saying [Helmfile is in its early days](https://github.com/helmfile/helmfile#status) in your README as... it's just untrue today.
