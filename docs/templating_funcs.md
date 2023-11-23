# Template Functions

#### `env`
The `env` function allows you to declare a particular environment variable as an optional for template rendering.
If the environment variable is unset or empty, the template rendering will continue with an empty string as a value.

```yaml
{{ $envValue := env "envName" }}
```

#### `requiredEnv`
The `requiredEnv` function allows you to declare a particular environment variable as required for template rendering.
If the environment variable is unset or empty, the template rendering will fail with an error message.

```yaml
{{ $envValue := requiredEnv "envName" }}
```

> If the environment variable value starts with '/' (forward slash) and [Git for Windows](https://git-scm.com/download/win) is used, you must set `MSYS_NO_PATHCONV=1` to preserve values as-is, or the environment variable value will be prefixed with the `C:\Program Files\Git`. [reference](https://github.com/git-for-windows/build-extra/blob/main/ReleaseNotes.md#known-issues)

#### `exec`
The `exec` function allows you to run a command, returning the stdout of the command. When the command fails, the template rendering will fail with an error message.

```yaml
{{ $cmdOutpot := exec "./mycmd" (list "arg1" "arg2" "--flag1") }}
```

#### `envExec`
The `envExec` function allows you to run a command with environment variables declared on-the-fly in addition to existing environment variables, returning the stdout of the command. When the command fails, the template rendering will fail with an error message.

```yaml
{{ $cmdOutpot := envExec (dict "envKey" "envValue") "./mycmd" (list "arg1" "arg2" "--flag1") }}
```

#### `isFile`
The `isFile` function allows you to check if a file exists. On failure, the template rendering will fail with an error message.

```yaml
{{ if isFile "./myfile" }}
```

#### `readFile`
The `readFile` function allows you to read a file and return its content as the function output. On failure, the template rendering will fail with an error message.

```yaml
{{ $fileContent := readFile "./myfile" }}
```

#### `readDir`
The `readDir` function returns a list of the relative paths to the files contained within the directory. (No folders included. Use `readDirEntries` if you need folders too)

```yaml
{{ range $index,$item := readDir "./testdata/tmpl/sample_folder/" }}
  {{- $itemSplit := splitList  "/" $item -}}
  {{- if contains "\\" $item -}}
  {{- $itemSplit = splitList "\\" $item -}}
  {{- end -}}
  {{- $itemValue := $itemSplit | last -}}
  {{- $itemValue -}}
{{- end -}}
```

#### `readDirEntries`
The `readDirEntries` function returns a list of [DirEntry](https://pkg.go.dev/os#DirEntry) contained within the directory

```yaml
{{ range $index,$item := readDirEntries "./testdata/tmpl/sample_folder/" }}
  {{- if $item.IsDir -}}
  {{- $item.Name -}}
  {{- end -}}
{{- end -}}
```

#### `toYaml`
The `toYaml` function allows you to convert a value to YAML string. When has failed, the template rendering will fail with an error message.

```yaml
{{ $yaml :=  $value | toYaml }}
```

#### `fromYaml`
The `fromYaml` function allows you to convert a YAML string to a value. When has failed, the template rendering will fail with an error message.

```yaml
{{ $value :=  $yamlString | fromYaml }}
```

#### `setValueAtPath`
The `setValueAtPath` function allows you to set a value at a path. When has failed, the template rendering will fail with an error message.

```yaml
{{ $value | setValueAtPath "path.key" $newValue }}
```

#### `get`
The `get` function allows you to get a value at a path. you can set a default value when the path is not found. When has failed, the template rendering will fail with an error message.

```yaml
{{ $Getvalue :=  $value | get "path.key" "defaultValue" }}
```

#### `getOrNil`
The `getOrNil` function allows you to get a value at a path. it will return nil when the value of path is not found. When has failed, the template rendering will fail with an error message.

```yaml
{{ $GetOrNlvalue :=  $value | getOrNil "path.key" }}
```

#### `tpl`
The `tpl` function allows you to render a template. When has failed, the template rendering will fail with an error message.

```yaml
{{ $tplValue :=  $value | tpl "{{ .Value.key }}" }}
```

#### `required`
The `required` function returns the second argument as-is only if it is not empty. If empty, the template rendering will fail with an error message containing the first argument.

```yaml
{{ $requiredValue :=  $value | required "value not set" }}
```

#### `fetchSecretValue`
The `fetchSecretValue` function parses the argument as a [vals](https://github.com/helmfile/vals) ref URL, retrieves and returns the remote secret value referred by the URL. In case it failed to access the remote secret backend for whatever reason or the URL was invalid, the template rendering will fail with an error message.

```yaml
{{ $fetchSecretValue :=  fetchSecretValue "secret/path" }}
```

#### `expandSecretRefs`
The `expandSecretRefs` function takes an object as the argument and expands every [vals](https://github.com/helmfile/vals) secret reference URL embedded in the object's values. See ["Remote Secrets" page in our documentation](./remote-secrets.md) for more information.

```yaml
{{ $expandSecretRefs :=  $value | expandSecretRefs }}
```
