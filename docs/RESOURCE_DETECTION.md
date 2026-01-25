# Resource Detection Based on Helm Template

This document describes the new resource detection feature based on Helm template manifest.

## Overview

The kubedog tracker now supports detecting resources by parsing Helm template output instead of querying the Kubernetes API. This approach has several advantages:

- No need to connect to a Kubernetes cluster
- Faster execution
- Works in dry-run and template modes
- Simpler and more reliable

## Usage

### Track Release from Manifest

```go
package main

import (
    "github.com/helmfile/helmfile/pkg/kubedog"
    "go.uber.org/zap"
)

func main() {
    logger := zap.NewExample().Sugar()
    
    config := &kubedog.TrackerConfig{
        Logger:       logger,
        Namespace:    "default",
        KubeContext:  "",
        Kubeconfig:   "",
        TrackOptions: kubedog.NewTrackOptions(),
    }
    
    tracker, err := kubedog.NewTracker(config)
    if err != nil {
        panic(err)
    }
    
    manifest := []byte(`
---
apiVersion: v1
kind: Service
metadata:
  name: my-service
  namespace: default
spec:
  selector:
    app: myapp
  ports:
  - port: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: myapp
        image: nginx:latest
`)
    
    err = tracker.TrackReleaseWithManifest(nil, "my-release", "default", manifest)
    if err != nil {
        logger.Errorf("Failed to track release: %v", err)
    }
    
    tracker.Close()
}
```

### Parse Manifest Directly

```go
import (
    "github.com/helmfile/helmfile/pkg/cluster"
)

func parseHelmOutput(manifest []byte) {
    releaseResources, err := cluster.GetReleaseResourcesFromManifest(
        manifest,
        "my-release",
        "default",
    )
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Found %d resources:\n", len(releaseResources.Resources))
    for _, res := range releaseResources.Resources {
        fmt.Printf("  - %s/%s in namespace %s\n", res.Kind, res.Name, res.Namespace)
    }
}
```

## Resource Classification

### Trackable Resources

Resources that need active tracking (wait for ready/completed):

- **Deployment** - Wait for all replicas to be ready
- **StatefulSet** - Wait for all replicas to be ready
- **DaemonSet** - Wait for desired number of scheduled nodes
- **Job** - Wait for job completion
- **Pod** - Wait for pod to be ready
- **ReplicaSet** - Wait for all replicas to be ready

### Static Resources

Resources that don't need active tracking (instantaneous creation):

- Service
- ConfigMap
- Secret
- PersistentVolume
- PersistentVolumeClaim
- StorageClass
- Namespace
- ResourceQuota
- LimitRange
- PriorityClass
- ServiceAccount
- Role
- RoleBinding
- ClusterRole
- ClusterRoleBinding
- NetworkPolicy
- Ingress
- CustomResourceDefinition

## Helper Functions

### Check if a resource kind is trackable

```go
isTrackable := cluster.IsTrackableKind("Deployment")
```

### Check if a resource kind is static

```go
isStatic := cluster.IsStaticKind("ConfigMap")
```

### Get Helm release labels

```go
labels := cluster.GetHelmReleaseLabels("my-release", "default")
// Returns:
// map[string]string{
//     "meta.helm.sh/release-name": "my-release",
//     "meta.helm.sh/release-namespace": "default",
// }
```

### Get Helm release annotations

```go
annotations := cluster.GetHelmReleaseAnnotations("my-release")
// Returns:
// map[string]string{
//     "meta.helm.sh/release-name": "my-release",
// }
```

## Integration with Helmfile

The tracker can be integrated with Helmfile to track releases after installation:

```go
func (st *HelmState) trackRelease(release *ReleaseSpec) error {
    if st.Tracker == nil {
        return nil
    }
    
    manifest, err := st.getManifest(release)
    if err != nil {
        return err
    }
    
    return st.Tracker.TrackReleaseWithManifest(
        context.Background(),
        release.Name,
        release.Namespace,
        manifest,
    )
}
```

## Advantages Over API-Based Detection

1. **No Cluster Access**: Works even without connecting to the cluster
2. **Faster**: No need to query multiple resource types via API
3. **Deterministic**: Always returns the same resources for the same manifest
4. **Offline Friendly**: Can be used for planning and validation
5. **Simpler**: Less complex error handling and retry logic

## Testing

The feature includes comprehensive tests:

```bash
# Run cluster package tests
go test ./pkg/cluster/... -v

# Run kubedog package tests
go test ./pkg/kubedog/... -v
```

## Implementation Details

- Uses `k8s.io/apimachinery/pkg/util/yaml` for parsing
- Handles multi-document YAML files (separated by `---`)
- Extracts resource kind, name, namespace, and manifest
- Skips resources without kind or name
- Returns empty resource list for empty manifests
