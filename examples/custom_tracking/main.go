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
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mysql-statefulset
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mysql
  template:
    metadata:
      labels:
        app: mysql
    spec:
      containers:
      - name: mysql
        image: mysql:5.7
`)

	fmt.Println("=== Example 1: Default tracking (all resources) ===")
	fmt.Println()

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

		fmt.Println()
	}

	fmt.Println("=== Example 2: Track only Deployments and StatefulSets ===")
	fmt.Println()

	trackConfig := &cluster.TrackConfig{
		TrackKinds: []string{"Deployment", "StatefulSet"},
	}

	releaseResources2, err := cluster.GetReleaseResourcesFromManifestWithConfig(
		manifest,
		"nginx-release",
		"default",
		trackConfig,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Found %d resources (filtered):\n\n", len(releaseResources2.Resources))

	for _, res := range releaseResources2.Resources {
		fmt.Printf("Kind: %s, Name: %s\n", res.Kind, res.Name)
	}

	fmt.Println()
	fmt.Println("=== Example 3: Skip ConfigMaps ===")
	fmt.Println()

	trackConfig2 := &cluster.TrackConfig{
		SkipKinds: []string{"ConfigMap"},
	}

	releaseResources3, err := cluster.GetReleaseResourcesFromManifestWithConfig(
		manifest,
		"nginx-release",
		"default",
		trackConfig2,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Found %d resources (filtered):\n\n", len(releaseResources3.Resources))

	for _, res := range releaseResources3.Resources {
		fmt.Printf("Kind: %s, Name: %s\n", res.Kind, res.Name)
	}

	fmt.Println()
	fmt.Println("=== Example 4: Custom trackable kinds (e.g., CronJob) ===")
	fmt.Println()

	cronJobManifest := []byte(`---
apiVersion: batch/v1beta1
kind: CronJob
metadata:
  name: my-cronjob
  namespace: default
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: hello
            image: busybox
            args:
            - /bin/sh
            - -c
            - date; echo Hello from Kubernetes cluster
`)

	trackConfig3 := &cluster.TrackConfig{
		CustomTrackableKinds: []string{"CronJob"},
	}

	releaseResources4, err := cluster.GetReleaseResourcesFromManifestWithConfig(
		cronJobManifest,
		"cron-release",
		"default",
		trackConfig3,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Found %d resources:\n\n", len(releaseResources4.Resources))

	for _, res := range releaseResources4.Resources {
		isTrackable := cluster.IsTrackableKindWithConfig(res.Kind, trackConfig3)
		fmt.Printf("Kind: %s, Name: %s, Trackable: %v\n", res.Kind, res.Name, isTrackable)
	}

	fmt.Println()
	fmt.Println("=== Example 5: Custom static kinds ===")
	fmt.Println()

	trackConfig4 := &cluster.TrackConfig{
		CustomStaticKinds: []string{"MyCustomResource"},
	}

	isStatic := cluster.IsStaticKindWithConfig("MyCustomResource", trackConfig4)
	fmt.Printf("Is 'MyCustomResource' static? %v\n", isStatic)

	isDefaultStatic := cluster.IsStaticKindWithConfig("ConfigMap", trackConfig4)
	fmt.Printf("Is 'ConfigMap' static (without custom config)? %v\n", isDefaultStatic)
}
