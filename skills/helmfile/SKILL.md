---
name: helmfile
description: Expert guidance for Helmfile declarative Helm chart deployment
---

# Helmfile Skill

You are an expert in Helmfile, a declarative spec for deploying Helm charts to Kubernetes clusters.

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

### Release Configuration Fields
| Field | Description |
|-------|-------------|
| `name` | Release name |
| `namespace` | Target namespace |
| `chart` | Chart reference (repo/chart or local path) |
| `version` | Semver constraint |
| `values` | Values files or inline values |
| `set`/`setString` | Override specific values |
| `secrets` | Encrypted values files (requires helm-secrets plugin) |
| `installed` | Set false to uninstall on sync |
| `wait` | Wait for resources to be ready |
| `timeout` | Operation timeout in seconds |
| `kubeContext` | Kubernetes context to use |
| `labels` | Key-value pairs for filtering |

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

## Templating

### Built-in Objects
- `.Environment.Name` - Current environment name
- `.Environment.KubeContext` - Environment's kube context
- `.Values` / `.StateValues` - Environment values
- `.Release.Name` - Release name
- `.Release.Namespace` - Release namespace
- `.Release.Labels` - Release labels
- `.Namespace` - Target namespace

### Template Functions
| Function | Description |
|----------|-------------|
| `env "VAR"` | Get optional env var (returns empty if unset) |
| `requiredEnv "VAR"` | Get required env var (fails if unset) |
| `exec "cmd" (list "args")` | Execute command |
| `readFile "path"` | Read file contents |
| `toYaml` / `fromYaml` | YAML conversion |
| `get .Values "key" default` | Get nested value with default |
| `required "msg" value` | Fail if value is empty |
| `fetchSecretValue "ref"` | Fetch secret from vals backend |
| `tpl "{{ .Value.key }}" .` | Render template string |

### Values Files Templates
Files ending with `.gotmpl` are rendered as templates:
```yaml
# values.yaml.gotmpl
db:
  username: {{ requiredEnv "DB_USERNAME" }}
  password: {{ requiredEnv "DB_PASSWORD" }}
```

## Environments

### Environment Configuration
```yaml
environments:
  default:
    values:
      - environments/default/values.yaml
  production:
    values:
      - environments/production/values.yaml
    secrets:
      - environments/production/secrets.yaml
    kubeContext: prod-cluster
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

## Advanced Features

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
    strategicMergePatches:
      - apiVersion: v1
        kind: ConfigMap
        metadata:
          name: raw1
        data:
          extra: value
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
releases:
  - name: app
    values:
      - db:
          password: ref+awssecrets://my-secret/db-password
```

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

### Git-based Charts
```yaml
repositories:
  - name: polaris
    url: git+https://github.com/reactiveops/polaris@deploy/helm?ref=master
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

### Hooks
```yaml
releases:
  - name: crds
    chart: ./crds
    hooks:
      - events: ["presync"]
        showlogs: true
        command: "kubectl"
        args: ["apply", "-f", "crds.yaml"]
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
