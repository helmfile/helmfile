---
name: helmfile
description: Expert guidance for Helmfile declarative Helm chart deployment
---

# Helmfile Skill

You are an expert in Helmfile, a declarative spec for deploying Helm charts to Kubernetes clusters.

## Status

Helmfile v1.0 and v1.1 have been released (May 2025). We recommend upgrading directly to v1.1 if you are still using v0.x. Helmfile supports both Helm 3.x and Helm 4.x.

## What is Helmfile

Helmfile is a declarative configuration tool that manages Helm releases. It allows you to:
- Keep a directory of chart value files and maintain changes in version control
- Apply CI/CD to configuration changes
- Periodically sync to avoid skew in environments

### Key Features
- **Declarative**: Write, version-control, apply desired state files
- **Modules**: Modularize common patterns, distribute via Git, S3
- **Versatility**: Manage charts, kustomizations, and Kubernetes resource directories
- **Patch**: JSON/Strategic-Merge Patch resources without forking charts

## Configuration Structure

### Quick Reference

A `helmfile.yaml` has these top-level sections:

| Section | Purpose |
|---------|---------|
| `repositories` | Helm chart repositories to use |
| `releases` | The Helm releases to deploy (core of helmfile) |
| `helmDefaults` | Default Helm options for all releases |
| `environments` | Environment-specific values (dev, staging, prod) |
| `helmfiles` | Include other helmfile.yaml files (nesting) |
| `bases` | Shared base files merged before this helmfile |
| `values` | Default values available in templates |
| `commonLabels` | Labels applied to all releases |
| `templates` | Reusable release templates |
| `hooks` | Global lifecycle hooks |
| `apiVersions` / `kubeVersion` | Kubernetes version capabilities |

### Basic helmfile.yaml
```yaml
repositories:
  - name: prometheus-community
    url: https://prometheus-community.github.io/helm-charts

releases:
  - name: prometheus
    namespace: monitoring
    chart: prometheus-community/prometheus
    version: "~19.0"
    values:
      - values.yaml
```

### Repository Configuration
```yaml
repositories:
  - name: stable
    url: https://charts.helm.sh/stable
  # Git-based repository
  - name: polaris
    url: git+https://github.com/reactiveops/polaris@deploy/helm?ref=master
  # OCI registry with auth
  - name: roboll
    url: roboll.io/charts
    certFile: optional_client_cert
    keyFile: optional_client_key
    username: optional_username
    password: optional_password
    oci: true
    passCredentials: true
    verify: true
    keyring: path/to/keyring.gpg
  # Self-signed certificate
  - name: insecure
    url: https://charts.example.com
    caFile: optional_ca_file
```

### Release Configuration Fields
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | | Release name |
| `namespace` | string | | Target namespace |
| `chart` | string | | Chart reference (repo/chart or local path) |
| `version` | string | | Semver constraint |
| `values` | list | | Values files or inline values |
| `set`/`setString` | list | | Override specific values |
| `secrets` | list | | Encrypted values files (requires helm-secrets plugin) |
| `installed` | bool | | Set false to uninstall on sync |
| `condition` | string | | Values lookup key for filtering releases |
| `wait` | bool | false | Wait for resources to be ready |
| `waitForJobs` | bool | false | Wait until all Jobs have completed |
| `timeout` | int | 300 | Operation timeout in seconds |
| `kubeContext` | string | | Kubernetes context to use |
| `labels` | map | | Key-value pairs for filtering |
| `createNamespace` | bool | true | Automatically create release namespace |
| `missingFileHandler` | string | | "Error" or "Warn" for missing files |
| `missingFileHandlerConfig` | map | | Additional missing file handler config |
| `valuesTemplate` | list | | Like `values` but template expressions rendered before passing to Helm |
| `setTemplate` | list | | Like `set` but template expressions rendered before passing to Helm |
| `apiVersions` | list | | Per-release API versions |
| `kubeVersion` | string | | Per-release kube version |
| `valuesPathPrefix` | string | | Prefix for values file paths |
| `verifyTemplate` | string | | Templated verify flag |
| `waitTemplate` | string | | Templated wait flag |
| `installedTemplate` | string | | Templated installed flag |
| `adopt` | list | | Resources to adopt (passes `--adopt` to Helm) |
| `forceGoGetter` | bool | false | Force go-getter URL parsing for chart field |
| `forceNamespace` | string | | Force namespace on all K8s resources |
| `skipRefresh` | bool | false | Per-release skip for `helm dependency up` |
| `disableAutoDetectedKubeVersionForDiff` | bool | false | Disable auto-detected kubeVersion for diff |
| `takeOwnership` | bool | false | Take ownership of existing resources |

### Helm Defaults
```yaml
helmDefaults:
  kubeContext: kube-context
  wait: true
  timeout: 600
  createNamespace: true
  force: false
  atomic: true
  cleanupOnFail: false
  verify: false
  keyring: path/to/keyring.gpg
  skipSchemaValidation: false
  waitForJobs: true
  recreatePods: false
  historyMax: 10
  devel: false
  skipDeps: false
  reuseValues: false
  enableDNS: false
  skipCRDs: false
  skipRefresh: false
  forceConflicts: false
  takeOwnership: false
  trackMode: ""
  disableAutoDetectedKubeVersionForDiff: false
  args:
    - "--set k=v"
  diffArgs:
    - "--suppress-secrets"
  syncArgs:
    - "--labels=app.kubernetes.io/managed-by=helmfile"
```

## CLI Commands

### Essential Commands
```bash
helmfile init                    # Initialize dependencies (helm, plugins)
helmfile sync                    # Sync cluster state (helm upgrade --install)
helmfile apply                   # Apply only when changes detected (diff + sync)
helmfile diff                    # Show differences before applying
helmfile destroy                 # Uninstall all releases
helmfile template                # Render manifests locally
helmfile lint                    # Lint charts
helmfile test                    # Run helm tests
helmfile list                    # List releases
helmfile deps                    # Lock dependencies
helmfile repos                   # Add chart repositories
helmfile fetch                   # Fetch charts from state file
helmfile status                  # Retrieve status of releases
helmfile build                   # Build all resources from state file
helmfile write-values            # Write values files (like template but for values)
helmfile unittest                # Unit test charts using helm-unittest plugin
helmfile show-dag                # Show release dependency graph (GROUP, RELEASE, DEPENDENCIES)
helmfile cache                   # Cache management
helmfile create                  # Create a helmfile deployment project scaffold
```

### Common Flags
| Flag | Description |
|------|-------------|
| `-f, --file` | Specify helmfile path |
| `-e, --environment` | Environment name (default: "default") |
| `-n, --namespace` | Override namespace |
| `-l, --selector` | Filter releases by labels |
| `--kube-context` | Kubernetes context |
| `--interactive` | Confirm before changes |
| `--skip-deps` | Skip dependency updates |
| `--allow-no-matching-release` | Don't error if selector has no matches |
| `-c, --chart` | Set chart (available in template as {{ .Chart }}) |
| `--color` | Output with color |
| `--debug` | Enable verbose output |
| `--state-values-set` | Override state values from CLI |
| `--state-values-file` | Override state values from file |
| `--track-mode` | Resource tracking mode (helm, helm-legacy, kubedog) |
| `--track-timeout` | Tracking timeout in seconds |
| `--track-logs` | Enable real-time log streaming |

### Fetch Command (Air-gapped Environments)
```bash
helmfile fetch --output-dir ./charts --write-output
```
| Flag | Default | Description |
|------|---------|-------------|
| `--output-dir` | temp dir | Directory to store charts |
| `--output-dir-template` | default template | Go template for output dir (`.OutputDir`, `.ChartName`, `.Release.*`, `.Environment.*`) |
| `--write-output` | false | Write helmfile.yaml with updated chart paths to stdout |
| `--concurrency` | 0 | Max concurrent helm processes |

### Show DAG
```bash
helmfile show-dag
```
Prints a table with GROUP, RELEASE, and DEPENDENCIES. Releases in the same GROUP are deployed concurrently. GROUP 2 starts only after GROUP 1 completes.

### Unit Tests
```bash
helmfile unittest                  # Requires helm-unittest plugin
```
```yaml
releases:
  - name: my-app
    chart: ./charts/my-app
    values:
      - values.yaml
    unitTests:
      - tests      # Relative to chart dir, /*_test.yaml appended
```

## Templating

### Built-in Objects
- `.Environment.Name` - Current environment name
- `.Environment.KubeContext` - Environment's kube context
- `.Values` / `.StateValues` - Environment values (`.StateValues` is an alias)
- `.Release.Name` - Release name
- `.Release.Namespace` - Release namespace
- `.Release.Labels` - Release labels
- `.Namespace` - Target namespace
- `.Chart` - Chart set via `--chart` flag
- `.HelmfileCommand` - The helmfile command being run

### Helmfile .Values vs Helm .Values
Helmfile uses the same `.Values` name as Helm. To distinguish, use `.StateValues` for Helmfile's values:
```yaml
app:
  project: {{.Environment.Name}}-{{.StateValues.project}}

{{`
extraEnvVars:
- name: APP_PROJECT
  value: {{.Values.app.project}}
`}}
```

### Template Functions
| Function | Description |
|----------|-------------|
| `env "VAR"` | Get optional env var (returns empty if unset) |
| `requiredEnv "VAR"` | Get required env var (fails if unset) |
| `exec "cmd" (list "args")` | Execute command, return stdout |
| `envExec (dict "k" "v") "cmd" (list "args")` | Execute command with custom env vars |
| `readFile "path"` | Read file contents |
| `readDir "path"` | List file paths in directory |
| `readDirEntries "path"` | List all entries including folders |
| `isFile "path"` | Check if file exists |
| `isDir "path"` | Check if directory exists |
| `toYaml` / `fromYaml` | YAML conversion |
| `get .Values "key" default` | Get nested value with default |
| `required "msg" value` | Fail if value is empty |
| `fetchSecretValue "ref"` | Fetch single secret from vals backend |
| `expandSecretRefs` | Fetch map of secrets from vals refs |
| `tpl "{{ .Value.key }}" .` | Render template string |

### Values Files Templates
Files ending with `.gotmpl` are rendered as templates:
```yaml
# values.yaml.gotmpl
db:
  username: {{ requiredEnv "DB_USERNAME" }}
  password: {{ requiredEnv "DB_PASSWORD" }}
```

### Template Partials
Files matching `_*.tpl` in the same directory are auto-loaded as helpers:
```
{{- define "myapp.labels" -}}
app: myapp
env: {{ .Environment.Name }}
{{- end -}}
```

## Environments

### Environment Configuration
```yaml
environments:
  default:
    values:
      - environments/default/values.yaml
      - environments/default/values.hcl
      - myChartVer: 1.0.0-dev
  production:
    values:
      - environments/production/values.yaml
      - myChartVer: 1.0.0
      - vault:
          enabled: false
    secrets:
      - environments/production/secrets.yaml
    kubeContext: prod-cluster
    missingFileHandler: Error
```

### Using Environments
```bash
helmfile -e production sync
helmfile --environment staging apply
```

### Conditional Releases
```yaml
releases:
  - name: monitoring
    installed: {{ eq .Environment.Name "production" }}
    chart: stable/prometheus
```

## Values Merging and Data Flow

Values are merged in this order (lowest to highest priority):

```
┌─────────────────────────────────────────────────────────────────┐
│                    VALUES MERGING ORDER                         │
├─────────────────────────────────────────────────────────────────┤
│  1. Base files (from `bases:`)                                  │
│  2. Root-level `values:` block (Defaults)                       │
│  3. Environment values (yaml/yaml.gotmpl)                       │
│  4. Environment values (HCL, including HCL secrets)             │
│  5. Environment secrets (non-HCL, decrypted)                    │
│  6. CLI overrides (--state-values-set, --state-values-file)     │
└─────────────────────────────────────────────────────────────────┘
```

**Later values override earlier values** at the map level (deep merge). Arrays use smart merging (sparse auto-detection by default).

```bash
# CLI overrides (highest priority)
helmfile --state-values-set image.tag=v2.0.0 sync
helmfile --state-values-file overrides.yaml sync
```

## Layering and Inheritance

### Bases (Layering)
```yaml
bases:
  - environments.yaml
  - defaults.yaml
  - templates.yaml
```

### Release Templates
```yaml
templates:
  default:
    namespace: kube-system
    missingFileHandler: Warn
    values:
      - config/{{`{{ .Release.Name }}`}}/values.yaml

releases:
  - name: app1
    inherit:
      - template: default
```

### Nested Helmfiles
```yaml
helmfiles:
  - path: releases/myapp/helmfile.yaml
    values:
      - {{ toYaml .Values | nindent 4 }}
```

## Hooks

### Hook Events
| Event | Description |
|-------|-------------|
| `prepare` | After release loaded from YAML, before execution |
| `preapply` | Before uninstall/install/upgrade during `apply` (only if changes exist) |
| `presync` | Before each release is synced (installed or upgraded) |
| `preuninstall` | Immediately before a release is uninstalled |
| `postuninstall` | After successful uninstall of a release |
| `postsync` | After each release is synced, regardless of success |
| `cleanup` | After each release is processed (counterpart to `prepare`) |

### Hook Configuration
```yaml
releases:
  - name: myapp
    chart: mychart
    hooks:
      - events: ["prepare", "cleanup"]
        showlogs: true
        command: "echo"
        args: ["{{`{{.Environment.Name}}`}}", "{{`{{.Release.Name}}`}}"]
      - events: ["presync"]
        showlogs: true
        command: "kubectl"
        args: ["apply", "-f", "crds.yaml"]
      - events: ["postsync"]
        showlogs: true
        command: "kubectl"
        args: ["rollout", "status", "deployment/myapp"]
```

### kubectlApply Hook
Alternative to `command`/`args`, directly apply manifests:
```yaml
hooks:
  - events: ["presync"]
    kubectlApply:
      - apiVersion: v1
        kind: ConfigMap
        metadata:
          name: my-config
        data:
          key: value
```

## Advanced Features

### Resource Tracking with Kubedog
```yaml
releases:
  - name: myapp
    chart: ./charts/myapp
    trackMode: kubedog
    trackTimeout: 300
    trackLogs: true
    trackKinds:
      - Deployment
      - StatefulSet
    skipKinds:
      - ConfigMap
    trackResources:
      - kind: Deployment
        name: myapp-deployment
        namespace: default
```

**Track Modes:**
| Mode | Description |
|------|-------------|
| `helm` (default) | Uses Helm's built-in `--wait` |
| `helm-legacy` | Uses Helm v4's `--wait=legacy` for compatibility |
| `kubedog` | Advanced tracking with detailed feedback |

### Kustomize Integration
Deploy kustomizations as Helm releases:
```yaml
releases:
  - name: myapp
    chart: ./kustomization-dir
    values:
      - images:
          - name: myapp
            newName: myregistry/myapp
            newTag: v1.0
```

### Strategic Merge Patches
```yaml
releases:
  - name: raw1
    chart: incubator/raw
    values:
      - resources:
        - apiVersion: v1
          kind: ConfigMap
          metadata:
            name: raw1
          data:
            foo: FOO
    strategicMergePatches:
      - apiVersion: v1
        kind: ConfigMap
        metadata:
          name: raw1
        data:
          bar: BAR
```

### JSON Patches
```yaml
releases:
  - name: myapp
    chart: mychart
    jsonPatches:
      - target:
          version: v1
          kind: ConfigMap
          name: myconfig
        patch:
          - op: remove
            path: /data/old-key
```

### Transformers
```yaml
releases:
  - name: app
    chart: mychart
    transformers:
      - apiVersion: builtin
        kind: LabelTransformer
        labels:
          env: production
        fieldSpecs:
          - path: metadata/labels
            create: true
```

### Chart Dependencies
Add dependencies without forking:
```yaml
releases:
  - name: foo
    chart: ./charts/foo
    dependencies:
      - chart: stable/envoy
        version: 1.5
        alias: sidecar
```

### Remote Secrets (vals)
```yaml
# Single key
releases:
  - name: app
    values:
      - db:
          password: {{ .Values.db.password | fetchSecretValue | quote }}

# Multiple keys
environments:
  default:
    values:
      - service:
          password: ref+vault://svc/#pass
          login: ref+vault://svc/#login
```

```yaml
# values.yaml.gotmpl
service:
{{ .Values.service | expandSecretRefs | toYaml | nindent 2 }}
```

Supported backends: Vault, AWS SSM, AWS Secrets Manager, GCP Secret Manager, Azure Key Vault, and more via [vals](https://github.com/helmfile/vals).

## Best Practices

### Directory Structure
```
.
├── helmfile.yaml
├── environments/
│   ├── default/
│   │   └── values.yaml
│   └── production/
│       ├── values.yaml
│       └── secrets.yaml
├── releases/
│   └── myapp/
│       └── helmfile.yaml
└── charts/
    └── mychart/
```

### DRY Configuration
1. Use `templates` for repeated release patterns
2. Use `bases` for shared configuration
3. Use `environments` for environment-specific values
4. Use `.gotmpl` files for templated values
5. Use `_*.tpl` partials for shared template logic

### Missing Keys Handling
```yaml
# Fail on missing key
{{ .Values.key }}

# Allow missing with default
{{ .Values | get "key" "default" }}
```

### Labels for Filtering
```yaml
commonLabels:
  team: platform

releases:
  - name: app1
    labels:
      tier: frontend
  - name: app2
    labels:
      tier: backend
```

```bash
helmfile -l tier=frontend sync
helmfile -l team=platform,tier=backend apply
```

## Common Patterns

### Multi-environment Setup
```yaml
environments:
  development:
    values:
      - replicas: 1
        resources: small
  production:
    values:
      - replicas: 3
        resources: large

releases:
  - name: myapp
    chart: mychart
    values:
      - replicaCount: {{ .Values.replicas }}
```

### OCI Charts
```yaml
repositories:
  - name: ghcr
    url: ghcr.io/myorg/charts
    oci: true

releases:
  - name: myapp
    chart: ghcr/myapp
    version: 1.0.0
```

## Troubleshooting

### Debug Template Rendering
```bash
helmfile build
helmfile template --debug
```

### Check Release Status
```bash
helmfile status
helmfile list
```

### Common Issues
1. **Missing env vars**: Use `requiredEnv` for required variables
2. **Chart not found**: Run `helmfile deps` or check repository config
3. **Diff plugin missing**: Install with `helm plugin install https://github.com/databus23/helm-diff`
4. **Secrets not decrypting**: Install helm-secrets plugin
5. **Helm v4 compatibility**: Use `trackMode: helm-legacy` for charts with broken `livenessProbe` configs

## Environment Variables
| Variable | Description |
|----------|-------------|
| `HELMFILE_ENVIRONMENT` | Default environment |
| `HELMFILE_TEMPDIR` | Temporary directory |
| `HELMFILE_CACHE_HOME` | Cache directory |
| `HELMFILE_DISABLE_INSECURE_FEATURES` | Security flag |
| `HELMFILE_EXPERIMENTAL` | Enable experimental features |

## When to Use This Skill

Invoke this skill when:
- Creating or modifying `helmfile.yaml` configurations
- Setting up multi-environment deployments
- Working with release templates and layering
- Integrating Kustomize with Helmfile
- Troubleshooting Helmfile deployment issues
- Implementing best practices for Helm chart management
- Configuring remote secrets with vals
- Using advanced features like strategic merge patches and transformers
- Setting up resource tracking with kubedog
- Managing Helm 4 compatibility
- Writing hooks for lifecycle management
