# Helmfile Agent Skill

Expert guidance for Helmfile, a declarative spec for deploying Helm charts to Kubernetes.

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

- **Configuration Structure**: Basic helmfile.yaml format and release configuration
- **CLI Commands**: sync, apply, diff, destroy, template, and more
- **Templating**: Built-in objects, template functions, values files templates
- **Environments**: Multi-environment setup and conditional releases
- **Layering**: Bases, release templates, nested helmfiles
- **Advanced Features**: Kustomize integration, strategic merge patches, transformers, chart dependencies, remote secrets
- **Best Practices**: Directory structure, DRY configuration, labels filtering
- **Troubleshooting**: Common issues and solutions

## Usage

Once installed, simply ask your AI agent questions about Helmfile:

- "Create a helmfile.yaml for deploying Prometheus"
- "How do I set up multi-environment deployments?"
- "Explain release templates and layering"
- "Help me troubleshoot a Helmfile sync issue"

## References

- [Helmfile Documentation](https://helmfile.readthedocs.io)
- [Helmfile GitHub](https://github.com/helmfile/helmfile)
- [Helm Documentation](https://helm.sh)
