package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		return appError("", fmt.Errorf("failed to resolve output directory: %w", err))
	}

	// Scaffold file paths (intermediate directories may not exist yet).
	helmfilePath := filepath.Join(absDir, "helmfile.yaml")
	envFilePath := filepath.Join(absDir, "environments", "default.yaml")
	gitkeepPath := filepath.Join(absDir, "values", ".gitkeep")

	// Preflight: when --force is not set, check all scaffold paths before
	// writing anything so the command fails atomically rather than leaving a
	// partially-written project directory.
	if !c.Force() {
		var existing []string
		for _, p := range []string{helmfilePath, envFilePath, gitkeepPath} {
			_, statErr := os.Stat(p)
			if statErr == nil {
				existing = append(existing, p)
			} else if !os.IsNotExist(statErr) {
				return appError("", fmt.Errorf("failed to check %s: %w", p, statErr))
			}
		}
		if len(existing) > 0 {
			return appError("", fmt.Errorf("the following files already exist, use --force to overwrite: %s", strings.Join(existing, ", ")))
		}
	}

	// Create directories.
	for _, dir := range []string{absDir, filepath.Join(absDir, "environments"), filepath.Join(absDir, "values")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return appError("", fmt.Errorf("failed to create directory %s: %w", dir, err))
		}
	}

	// Write scaffold files.
	files := []struct {
		path    string
		content []byte
	}{
		{helmfilePath, []byte(helmfileYAMLTemplate)},
		{envFilePath, []byte(envDefaultYAMLTemplate)},
		{gitkeepPath, []byte("")},
	}
	for _, f := range files {
		if err := writeScaffoldFile(f.path, f.content, c.Force()); err != nil {
			return appError("", fmt.Errorf("failed to write %s: %w", f.path, err))
		}
		c.Logger().Infof("created %s", f.path)
	}

	c.Logger().Infof("\nhelmfile project created in %s\n\nNext steps:\n  cd %s\n  # Edit helmfile.yaml to add your releases\n  helmfile apply", absDir, absDir)
	return nil
}

// writeScaffoldFile writes content to path. When force is false it uses
// O_EXCL so that a file appearing between the preflight check and the write
// is caught rather than silently overwritten (TOCTOU protection).
func writeScaffoldFile(path string, content []byte, force bool) error {
	if force {
		return os.WriteFile(path, content, 0o644)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("file %s already exists, use --force to overwrite: %w", path, err)
		}
		return err
	}
	_, werr := f.Write(content)
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}
