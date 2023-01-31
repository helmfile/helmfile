# helmfile template built-in objects

- `Environment`: The information about the environment. This is set by the
  `--environment` flag. It has several objects inside of it:
  - `Environment.Name`: The name of the environment
- `Path`: The file path to the current helmfile.
  - `Path.Dir`: The name of the directory in which the current helmfile resides
  - `Path.Base`: The file name, or the last element of the path of the current helmfile file
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
- `Path`: The file path to the current helmfile.
  - `Path.Dir`: The name of the directory in which the current helmfile resides
  - `Path.Base`: The file name, or the last element of the path of the current helmfile file

The built-in values always begin with a capital letter. This is in keeping with
Go's naming convention. When you define your own values and template variables, you are free to use a
convention that suits your team.