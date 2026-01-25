# Custom Resource Tracking

This document describes how to configure custom resource tracking in the kubedog tracker.

## Overview

The kubedog tracker now supports flexible configuration for resource tracking through the `TrackOptions` struct. You can:

- **Track specific kinds**: Only track resources of specified types
- **Skip specific kinds**: Exclude certain resource types from tracking
- **Define custom trackable kinds**: Add new resource types that should be actively tracked
- **Define custom static kinds**: Add new resource types that don't need active tracking

## Configuration Options

### TrackKinds

When set, only resources in this list will be tracked. All other resources are ignored.

**Example:**
```go
opts := NewTrackOptions().WithTrackKinds([]string{"Deployment", "StatefulSet"})
```

This configuration will:
- Track only `Deployment` and `StatefulSet` resources
- Ignore all other resource types (Service, ConfigMap, etc.)

### SkipKinds

Resources in this list will be skipped, even if they would normally be tracked.

**Example:**
```go
opts := NewTrackOptions().WithSkipKinds([]string{"ConfigMap", "Secret"})
```

This configuration will:
- Track all normally trackable resources (Deployment, StatefulSet, etc.)
- Skip `ConfigMap` and `Secret` resources

### CustomTrackableKinds

Define additional resource types that should be actively tracked. When configured, only these custom types and resources in `TrackKinds` (if set) will be considered trackable.

**Example:**
```go
opts := NewTrackOptions().WithCustomTrackableKinds([]string{"CronJob", "ReplicationController"})
```

This configuration will:
- Treat `CronJob` and `ReplicationController` as trackable resources
- Ignore default trackable kinds (Deployment, StatefulSet, etc.) unless also in `TrackKinds`

### CustomStaticKinds

Define additional resource types that are considered static and don't need active tracking.

**Example:**
```go
opts := NewTrackOptions().WithCustomStaticKinds([]string{"NetworkPolicy", "PodDisruptionBudget"})
```

This configuration will:
- Treat `NetworkPolicy` and `PodDisruptionBudget` as static resources
- Ignore default static kinds unless not in this list

## Usage Examples

### Example 1: Track Only Deployments and StatefulSets

```go
package main

import (
    "github.com/helmfile/helmfile/pkg/kubedog"
    "go.uber.org/zap"
)

func main() {
    // Configure tracker to only track Deployments and StatefulSets
    opts := kubedog.NewTrackOptions().
        WithTrackKinds([]string{"Deployment", "StatefulSet"}).
        WithTimeout(600)

    config := &kubedog.TrackerConfig{
        Logger:       zap.NewExample().Sugar(),
        Namespace:    "default",
        KubeContext:  "",
        Kubeconfig:   "",
        TrackOptions: opts,
    }

    tracker, err := kubedog.NewTracker(config)
    if err != nil {
        panic(err)
    }

    manifest := []byte(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
  namespace: default
spec:
  replicas: 3
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
kind: StatefulSet
metadata:
  name: my-statefulset
  namespace: default
spec:
  replicas: 1
`)

    // This will track Deployment and StatefulSet, but skip Service
    err = tracker.TrackReleaseWithManifest(nil, "my-release", "default", manifest)
    if err != nil {
        panic(err)
    }

    tracker.Close()
}
```

### Example 2: Skip ConfigMaps and Secrets

```go
opts := kubedog.NewTrackOptions().
    WithSkipKinds([]string{"ConfigMap", "Secret"})

// This will track all normally trackable resources (Deployment, StatefulSet, etc.)
// but skip ConfigMap and Secret resources
```

### Example 3: Add Custom Trackable Kind (CronJob)

```go
opts := kubedog.NewTrackOptions().
    WithCustomTrackableKinds([]string{"CronJob"})

// This will treat CronJob as a trackable resource and wait for it
// Default trackable kinds (Deployment, StatefulSet) will not be tracked
```

### Example 4: Combined Configuration

```go
opts := kubedog.NewTrackOptions().
    WithTrackKinds([]string{"Deployment", "CronJob"}).
    WithSkipKinds([]string{"ConfigMap"})

// This configuration:
// 1. Only tracks Deployment and CronJob resources
// 2. Skips ConfigMap even if it appears in the manifest
// 3. Ignores all other resource types
```

## Priority and Behavior

The tracker evaluates configuration in the following order:

1. **SkipKinds**: If a resource kind is in SkipKinds, it's immediately skipped
2. **TrackKinds**: If TrackKinds is set, only resources in this list are considered
3. **CustomTrackableKinds / CustomStaticKinds**: 
   - If CustomTrackableKinds is set, only these kinds are considered trackable
   - If CustomStaticKinds is set, only these kinds are considered static
   - Otherwise, default trackable/static lists are used

## Resource Classification

### Default Trackable Kinds

These resources are actively tracked by default:
- Deployment
- StatefulSet
- DaemonSet
- Job
- Pod
- ReplicaSet

### Default Static Kinds

These resources don't need active tracking by default:
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

## Integration with Helmfile

When using kubedog tracker with Helmfile, you can configure tracking options in your helmfile.yaml:

```yaml
releases:
- name: my-app
  namespace: default
  chart: ./charts/my-app
  track:
    timeout: 600
    trackKinds:
      - Deployment
      - StatefulSet
    skipKinds:
      - ConfigMap
```

## See Also

- [Resource Detection Guide](./RESOURCE_DETECTION.md)
- [Helmfile README](../README.md)
