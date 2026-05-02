package app

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	helmfileYAMLTemplate = `# Helmfile configuration
# Documentation: https://helmfile.readthedocs.io/

# Common Helm defaults applied to all releases
helmDefaults:
  createNamespace: true
  wait: true
  timeout: 300

# # Helm chart repositories
# repositories:
#   - name: bitnami
#     url: https://charts.bitnami.com/bitnami
#   - name: ingress-nginx
#     url: https://kubernetes.github.io/ingress-nginx
#   - name: prometheus-community
#     url: https://prometheus-community.github.io/helm-charts

# # Environment-specific values
# # Usage: helmfile -e <environment> apply
# environments:
#   default:
#     values:
#       - environments/default.yaml
#   staging:
#     values:
#       - environments/staging.yaml
#   production:
#     values:
#       - environments/production.yaml

# # Helm releases
# releases:
#   - name: my-app
#     namespace: my-app
#     chart: bitnami/nginx
#     version: ~18.0.0
#     values:
#       - values/my-app.yaml
#     # secrets:
#     #   - secrets/my-app.yaml
#     # hooks:
#     #   - events: ["presync"]
#     #     command: kubectl
#     #     args: ["apply", "-f", "manifests/"]
`

	envDefaultYAMLTemplate = `# Default environment values
# These values are available in helmfile.yaml as {{ .Values }}
# Example:
# replicaCount: 1
# image:
#   repository: nginx
#   tag: latest
`
)

func (a *App) Create(c CreateConfigProvider) error {
	outputDir := c.OutputDir()
	absDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("failed to resolve output directory: %w", err)
	}

	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", absDir, err)
	}

	helmfilePath := filepath.Join(absDir, "helmfile.yaml")
	if err := os.WriteFile(helmfilePath, []byte(helmfileYAMLTemplate), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", helmfilePath, err)
	}
	c.Logger().Infof("created %s", helmfilePath)

	envDir := filepath.Join(absDir, "environments")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", envDir, err)
	}

	envFilePath := filepath.Join(envDir, "default.yaml")
	if err := os.WriteFile(envFilePath, []byte(envDefaultYAMLTemplate), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", envFilePath, err)
	}
	c.Logger().Infof("created %s", envFilePath)

	valuesDir := filepath.Join(absDir, "values")
	if err := os.MkdirAll(valuesDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", valuesDir, err)
	}

	gitkeepPath := filepath.Join(valuesDir, ".gitkeep")
	if err := os.WriteFile(gitkeepPath, []byte(""), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", gitkeepPath, err)
	}
	c.Logger().Infof("created %s", gitkeepPath)

	c.Logger().Infof("\nhelmfile project created in %s\n\nNext steps:\n  cd %s\n  # Edit helmfile.yaml to add your releases\n  helmfile apply", absDir, absDir)
	return nil
}
