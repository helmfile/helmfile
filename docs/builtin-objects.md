# helmfile template built-in objects

- `Environment`: The information about the environment. This is set by the
  `--environment` flag. It has several objects inside of it:
  - `Environment.Name`: The name of the environment
- `Values`: Values passed into the environment.
- `StateValues`: alias for `Values`.
- `Namespace`: The namespace to be released into

# release template built-in objects

it be used for the below tow cases:

1. release specific template
```
templates:
  default:
    chart: stable/{{`{{ .Release.Name }}`}}
    namespace: kube-system
    values:
    - config/{{`{{ .Release.Name }}`}}/values.yaml
    - config/{{`{{ .Release.Name }}`}}/{{`{{ .Environment.Name }}`}}.yaml
    secrets:
    - config/{{`{{ .Release.Name }}`}}/secrets.yaml
    - config/{{`{{ .Release.Name }}`}}/{{`{{ .Environment.Name }}`}}-secrets.yaml
releases:
- name: heapster
  version: 0.3.2
  inherit:
    template: default
- name: kubernetes-dashboard
  version: 0.10.0
  inherit:
    template: default
```

2. release values template
```
releases:
- name: some-release
  chart: my-chart
  values:
    # This is a template file can use the built-in objects 
    - path/to/values.gotmpl
```

- `Release`: This object describes the release itself. It has several objects
  inside of it:
  - `Release.Name`: The release name
  - `Release.Namespace`: The namespace to be released into
  - `Release.Labels`: The labels to be applied to the release
  - `Release.Chart`: The chart name of the release
  - `Release.KubeContext`: The kube context to be used for the release
- `Values`: Values passed into the environment.
- `StateValues`: alias for `Values`.
- `Environment`: The information about the environment. This is set by the
  `--environment` flag. It has several objects inside of it:
  - `Environment.Name`: The name of the environment
- `Chart`: The chart name for the release.
- `KubeContext`: The kube context to be used for the release
- `Namespace`: The namespace to be released into

The built-in values always begin with a capital letter. This is in keeping with
Go's naming convention. When you define your own values and template variables, you are free to use a
convention that suits your team.