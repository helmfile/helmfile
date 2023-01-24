# releaseset template built-in objects

- `Environment`: The information about the environment. This is set by the
  `--environment` flag. It has several objects inside of it:
  - `Environment.Name`: The name of the environment
- `StateFile`: The information about the state file. It has several objects
  inside of it:
  - `StateFile.Name`: The name of the current state file
  - `StateFile.BasePath`: The base path of the current state file
- `RootStateFile`: The information about the root state file. It has several objects
  inside of it:
  - `StateFile.Path`: The path of the root state file
- `Values`: Values passed into the environment.
- `StateValues`: alias for `Values`.
- `Namespace`: The namespace to be released into

# release template built-in objects

it be used for the below case:
```
apiVersion: v1
kind: ConfigMap
metadata:
  # release template
  name: {{`{{ .Release.Name }}`}}-1
  namespace: {{`{{ .Release.Namespace }}`}}
data:
  foo: FOO
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
- `StateFile`: The information about the state file. It has several objects
  inside of it:
  - `StateFile.Name`: The name of the current state file
  - `StateFile.BasePath`: The base path of the current state file
- `RootStateFile`: The information about the root state file. It has several objects
  inside of it:
  - `StateFile.Path`: The path of the root state file

The built-in values always begin with a capital letter. This is in keeping with
Go's naming convention. When you define your own values and template variables, you are free to use a
convention that suits your team.