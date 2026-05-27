# Helmfile Agent Skill

Expert guidance for Helmfile v1.1, a declarative spec for deploying Helm charts to Kubernetes.

## Installation

### Using skills.sh CLI

```bash
npx skills add helmfile/helmfile --skill helmfile
```

### Manual Installation

#### For Claude Code
```bash
mkdir -p ~/.claude/skills
cp -r skills/helmfile ~/.claude/skills/
```

#### For Cursor
```bash
mkdir -p ~/.cursor/skills
cp -r skills/helmfile ~/.cursor/skills/
```

#### For OpenCode
```bash
mkdir -p ~/.agents/skills
cp -r skills/helmfile ~/.agents/skills/
```

## What This Skill Covers

- **Status**: Helmfile v1.0/v1.1 released, supports Helm 3.x and Helm 4.x
- **Configuration Structure**: Full helmfile.yaml reference with all release fields
- **CLI Commands**: sync, apply, diff, destroy, template, fetch, unittest, show-dag, write-values, and more
- **Templating**: Built-in objects, template functions (env, exec, readFile, fetchSecretValue, expandSecretRefs), partials
- **Environments**: Multi-environment setup, HCL values, conditional releases
- **Values Merging**: Data flow and precedence (bases -> root values -> env values -> HCL -> secrets -> CLI overrides)
- **Layering**: Bases, release templates, nested helmfiles
- **Hooks**: Lifecycle hooks (prepare, preapply, presync, preuninstall, postuninstall, postsync, cleanup) with kubectlApply
- **Advanced Features**: Kubedog resource tracking, Kustomize integration, strategic merge patches, JSON patches, transformers, chart dependencies, remote secrets (vals)
- **Best Practices**: Directory structure, DRY configuration, labels filtering, missing keys handling
- **Troubleshooting**: Common issues and solutions, Helm 4 compatibility

## Usage

Once installed, simply ask your AI agent questions about Helmfile:

- "Create a helmfile.yaml for deploying Prometheus"
- "How do I set up multi-environment deployments?"
- "Explain release templates and layering"
- "Help me troubleshoot a Helmfile sync issue"
- "How do I use kubedog for resource tracking?"
- "Set up hooks for CRD installation before sync"
- "How do I use vals for remote secrets?"

## References

- [Helmfile Documentation](https://helmfile.readthedocs.io)
- [Helmfile GitHub](https://github.com/helmfile/helmfile)
- [Helm Documentation](https://helm.sh)
- [vals - Secret References](https://github.com/helmfile/vals)
- [kubedog - Resource Tracking](https://github.com/werf/kubedog)
