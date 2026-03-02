package resource

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

func ParseManifest(manifest []byte, defaultNamespace string, logger *zap.SugaredLogger) ([]Resource, error) {
	var resources []Resource

	decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)

	for {
		var obj unstructured.Unstructured
		err := decoder.Decode(&obj)
		if err != nil {
			if errors.Is(err, io.EOF) {
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
			namespace = defaultNamespace
		}

		resources = append(resources, Resource{
			Kind:      kind,
			Name:      name,
			Namespace: namespace,
		})
	}

	return resources, nil
}
