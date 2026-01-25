package cluster

import (
	"bytes"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

type Resource struct {
	Kind      string
	Name      string
	Namespace string
	Manifest  string
}

type ReleaseResources struct {
	ReleaseName string
	Namespace   string
	Resources   []Resource
}

type TrackConfig struct {
	TrackKinds           []string
	SkipKinds            []string
	CustomTrackableKinds []string
	CustomStaticKinds    []string
}

func GetReleaseResourcesFromManifest(manifest []byte, releaseName, releaseNamespace string) (*ReleaseResources, error) {
	return GetReleaseResourcesFromManifestWithLogger(nil, manifest, releaseName, releaseNamespace, nil)
}

func GetReleaseResourcesFromManifestWithConfig(manifest []byte, releaseName, releaseNamespace string, config *TrackConfig) (*ReleaseResources, error) {
	return GetReleaseResourcesFromManifestWithLogger(nil, manifest, releaseName, releaseNamespace, config)
}

func GetReleaseResourcesFromManifestWithLogger(logger *zap.SugaredLogger, manifest []byte, releaseName, releaseNamespace string, config *TrackConfig) (*ReleaseResources, error) {
	resources, err := parseManifest(manifest, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	if len(resources) == 0 {
		if logger != nil {
			logger.Debugf("No resources found in manifest for release %s", releaseName)
		}
		return &ReleaseResources{
			ReleaseName: releaseName,
			Namespace:   releaseNamespace,
			Resources:   resources,
		}, nil
	}

	if config != nil {
		filteredResources := filterResourcesByConfig(resources, config, logger)
		if logger != nil {
			logger.Infof("Found %d resources in manifest for release %s (filtered from %d total)", len(filteredResources), releaseName, len(resources))
			for _, res := range filteredResources {
				logger.Debugf("  - %s/%s in namespace %s", res.Kind, res.Name, res.Namespace)
			}
		}
		return &ReleaseResources{
			ReleaseName: releaseName,
			Namespace:   releaseNamespace,
			Resources:   filteredResources,
		}, nil
	}

	if logger != nil {
		logger.Infof("Found %d resources in manifest for release %s", len(resources), releaseName)
		for _, res := range resources {
			logger.Debugf("  - %s/%s in namespace %s", res.Kind, res.Name, res.Namespace)
		}
	}

	return &ReleaseResources{
		ReleaseName: releaseName,
		Namespace:   releaseNamespace,
		Resources:   resources,
	}, nil
}

func filterResourcesByConfig(resources []Resource, config *TrackConfig, logger *zap.SugaredLogger) []Resource {
	var filtered []Resource

	for _, res := range resources {
		if shouldSkipResource(res.Kind, config, logger) {
			if logger != nil {
				logger.Debugf("Skipping resource %s/%s (kind: %s) based on configuration", res.Kind, res.Name, res.Kind)
			}
			continue
		}
		filtered = append(filtered, res)
	}

	return filtered
}

func shouldSkipResource(kind string, config *TrackConfig, logger *zap.SugaredLogger) bool {
	if len(config.TrackKinds) > 0 {
		shouldTrack := false
		for _, trackKind := range config.TrackKinds {
			if kind == trackKind {
				shouldTrack = true
				break
			}
		}
		if !shouldTrack {
			if logger != nil {
				logger.Debugf("Resource kind %s is not in TrackKinds list, skipping", kind)
			}
			return true
		}
	}

	if len(config.SkipKinds) > 0 {
		for _, skipKind := range config.SkipKinds {
			if kind == skipKind {
				if logger != nil {
					logger.Debugf("Resource kind %s is in SkipKinds list, skipping", kind)
				}
				return true
			}
		}
	}

	return false
}

func parseManifest(manifest []byte, logger *zap.SugaredLogger) ([]Resource, error) {
	var resources []Resource

	decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)

	for {
		var obj unstructured.Unstructured
		err := decoder.Decode(&obj)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("failed to decode manifest: %w", err)
		}

		if len(obj.Object) == 0 {
			continue
		}

		kind := obj.GetKind()
		if kind == "" {
			if logger != nil {
				logger.Debugf("Skipping resource without kind")
			}
			continue
		}

		name := obj.GetName()
		if name == "" {
			if logger != nil {
				logger.Debugf("Skipping %s resource without name", kind)
			}
			continue
		}

		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}

		manifestBytes, err := yaml.Marshal(obj.Object)
		if err != nil {
			if logger != nil {
				logger.Debugf("Failed to marshal %s/%s: %v", kind, name, err)
			}
			continue
		}

		res := Resource{
			Kind:      kind,
			Name:      name,
			Namespace: namespace,
			Manifest:  string(manifestBytes),
		}
		resources = append(resources, res)
	}

	return resources, nil
}

func IsTrackableKind(kind string) bool {
	trackableKinds := map[string]bool{
		"Deployment":  true,
		"StatefulSet": true,
		"DaemonSet":   true,
		"Job":         true,
		"Pod":         true,
		"ReplicaSet":  true,
	}
	return trackableKinds[kind]
}

func IsTrackableKindWithConfig(kind string, config *TrackConfig) bool {
	if config == nil {
		return IsTrackableKind(kind)
	}

	if len(config.CustomTrackableKinds) > 0 {
		for _, customKind := range config.CustomTrackableKinds {
			if kind == customKind {
				return true
			}
		}
		return false
	}

	return IsTrackableKind(kind)
}

func IsStaticKind(kind string) bool {
	staticKinds := map[string]bool{
		"Service":                  true,
		"ConfigMap":                true,
		"Secret":                   true,
		"PersistentVolumeClaim":    true,
		"PersistentVolume":         true,
		"StorageClass":             true,
		"Namespace":                true,
		"ResourceQuota":            true,
		"LimitRange":               true,
		"PriorityClass":            true,
		"ServiceAccount":           true,
		"Role":                     true,
		"RoleBinding":              true,
		"ClusterRole":              true,
		"ClusterRoleBinding":       true,
		"NetworkPolicy":            true,
		"Ingress":                  true,
		"CustomResourceDefinition": true,
	}
	return staticKinds[kind]
}

func IsStaticKindWithConfig(kind string, config *TrackConfig) bool {
	if config == nil {
		return IsStaticKind(kind)
	}

	if len(config.CustomStaticKinds) > 0 {
		for _, customKind := range config.CustomStaticKinds {
			if kind == customKind {
				return true
			}
		}
		return false
	}

	return IsStaticKind(kind)
}

func GetHelmReleaseLabels(releaseName, releaseNamespace string) map[string]string {
	return map[string]string{
		"meta.helm.sh/release-name":      releaseName,
		"meta.helm.sh/release-namespace": releaseNamespace,
	}
}

func GetHelmReleaseAnnotations(releaseName string) map[string]string {
	return map[string]string{
		"meta.helm.sh/release-name": releaseName,
	}
}
