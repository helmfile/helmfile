package main

import (
	"fmt"

	"github.com/helmfile/helmfile/pkg/cluster"
)

func main() {
	// Example manifest from helm template command
	manifest := []byte(`---
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
  namespace: default
spec:
  selector:
    app: nginx
  ports:
  - port: 80
    targetPort: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-config
  namespace: default
data:
  nginx.conf: |
    server {
      listen 8080;
      location / {
        root /usr/share/nginx/html;
      }
    }
`)

	// Parse the manifest
	releaseResources, err := cluster.GetReleaseResourcesFromManifest(
		manifest,
		"nginx-release",
		"default",
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Release: %s\n", releaseResources.ReleaseName)
	fmt.Printf("Namespace: %s\n", releaseResources.Namespace)
	fmt.Printf("Found %d resources:\n\n", len(releaseResources.Resources))

	for _, res := range releaseResources.Resources {
		isTrackable := cluster.IsTrackableKind(res.Kind)
		isStatic := cluster.IsStaticKind(res.Kind)

		fmt.Printf("Kind: %s\n", res.Kind)
		fmt.Printf("Name: %s\n", res.Name)
		fmt.Printf("Namespace: %s\n", res.Namespace)

		if isTrackable {
			fmt.Printf("Status: Trackable (needs waiting for ready)\n")
		} else if isStatic {
			fmt.Printf("Status: Static (no tracking needed)\n")
		} else {
			fmt.Printf("Status: Unknown\n")
		}

		fmt.Printf("\n")
	}

	// Get Helm labels
	labels := cluster.GetHelmReleaseLabels("nginx-release", "default")
	fmt.Println("Helm Labels:")
	for k, v := range labels {
		fmt.Printf("  %s: %s\n", k, v)
	}
}
