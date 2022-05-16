# Template Functions

#### `requiredEnv` 
The `requiredEnv` function allows you to declare a particular environment variable as required for template rendering.
If the environment variable is unset or empty, the template rendering will fail with an error message.

```yaml
{{ $envValue := requiredEnv "envName" }}
```

#### `exec`
The `exec` function allows you to run a command, returning the stdout of the command. When the command fails, the template rendering will fail with an error message.

```yaml
{{ $cmdOutpot := exec "./mycmd" (list "arg1" "arg2" "--flag1") }}
```

#### `readFile`
The `readFile` function allows you to read a file and return its content as the function output. On failure, the template rendering will fail with an error message.

```yaml
{{ $fileContent := readFile "./myfile" }}
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
The `get` function allows you to get a value at a path. when defaultValue not set. It will return nil. When has failed, the template rendering will fail with an error message.

```yaml
{{ $Getvalue :=  $value | get "path.key" "defaultValue" }}
```

#### `getOrNil`
The `getOrNil` function allows you to get a value at a path. when defaultValue not set. It will return nil. When has failed, the template rendering will fail with an error message.

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
The `fetchSecretValue` function parses the argument as a [vals](https://github.com/variantdev/vals) ref URL, retrieves and returns the remote secret value referred by the URL. In case it failed to access the remote secret backend for whatever reason or the URL was invalid, the template rendering will fail with an error message.

```yaml
{{ $fetchSecretValue :=  fetchSecretValue "secret/path" }}
```

### `expandSecretRefs`
The `expandSecretRefs` function takes an object as the argument and expands every [vals](https://github.com/variantdev/vals) secret reference URL embedded in the object's values. See ["Remote Secrets" page in our documentation](./templating_funcs.md) for more information.

```yaml
{{ $expandSecretRefs :=  $value | expandSecretRefs }}
```