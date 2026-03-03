# Kubedog Configuration

This document describes how to configure kubedog resource tracking in Helmfile.

## Overview

Kubedog is a library for tracking Kubernetes resources during deployments. Helmfile uses kubedog when `trackMode: kubedog` is set to monitor the rollout of resources like Deployments, StatefulSets, DaemonSets, and Jobs.

## Configuration Options

### Release-level Configuration

You can configure kubedog settings per release:

```yaml
releases:
  - name: my-app
    namespace: default
    chart: my-chart
    trackMode: kubedog
    kubedogQPS: 100      # Queries per second (default: 100)
    kubedogBurst: 200    # Burst capacity (default: 200)
    trackLogs: true
    trackKinds:
      - Deployment
```

### Global Default Configuration

You can also set defaults in `helmDefaults`:

```yaml
helmDefaults:
  trackMode: kubedog
  # Note: QPS and Burst can only be configured at release level
```

## Parameters

### kubedogQPS

- **Type**: `float32`
- **Default**: `100`
- **Description**: Sets the maximum number of queries per second to the Kubernetes API server from the kubedog client. This controls the rate of API requests when tracking resources.

**When to increase**:
- Large clusters with many resources
- When tracking multiple releases simultaneously
- When you see rate limiting errors like "client rate limiter Wait returned an error: context canceled"

**When to decrease**:
- Small clusters or development environments
- When you want to reduce load on the API server

### kubedogBurst

- **Type**: `int`
- **Default**: `200`
- **Description**: Sets the maximum burst of requests that can be made to the Kubernetes API server. This allows temporary spikes above the QPS limit.

**When to increase**:
- When tracking releases with many resources
- When you see connection timeout errors
- In production environments with high throughput needs

**When to decrease**:
- In resource-constrained environments
- When API server is under heavy load

## Tuning Guidelines

### For Small Clusters (< 50 resources)

```yaml
releases:
  - name: my-app
    trackMode: kubedog
    kubedogQPS: 50
    kubedogBurst: 100
```

### For Medium Clusters (50-200 resources)

```yaml
releases:
  - name: my-app
    trackMode: kubedog
    kubedogQPS: 100  # default
    kubedogBurst: 200  # default
```

### For Large Clusters (> 200 resources)

```yaml
releases:
  - name: my-app
    trackMode: kubedog
    kubedogQPS: 200
    kubedogBurst: 400
```

### For Multiple Concurrent Releases

When using `--concurrent` flag with multiple releases that use kubedog tracking:

```yaml
releases:
  - name: app1
    trackMode: kubedog
    kubedogQPS: 50
    kubedogBurst: 100
    
  - name: app2
    trackMode: kubedog
    kubedogQPS: 50
    kubedogBurst: 100
```

## Troubleshooting

### Rate Limiting Errors

**Error**:
```
E0302 19:38:41.812322 91 reflector.go:204] "Failed to watch" err="client rate limiter Wait returned an error: context canceled"
```

**Solution**: Increase `kubedogQPS` and `kubedogBurst` values.

### Connection Timeouts

**Error**:
```
context canceled while waiting for API server response
```

**Solution**: 
1. Check network connectivity to the API server
2. Increase `kubedogBurst` to allow more concurrent requests
3. Decrease number of concurrent releases if using `--concurrent` flag

### Slow Tracking

**Symptom**: Resource tracking takes a long time to complete.

**Solution**: 
1. Use `trackKinds` to limit which resource types are tracked
2. Use `skipKinds` to exclude unnecessary resource types
3. Increase `kubedogQPS` to speed up API queries

## Related Configuration

### trackTimeout

Sets the timeout for kubedog tracking (in seconds):

```yaml
releases:
  - name: my-app
    trackMode: kubedog
    trackTimeout: 600  # 10 minutes
```

### trackLogs

Enable/disable log streaming from tracked resources:

```yaml
releases:
  - name: my-app
    trackMode: kubedog
    trackLogs: true  # Show pod logs during tracking
```

### trackKinds / skipKinds

Control which resource types to track:

```yaml
releases:
  - name: my-app
    trackMode: kubedog
    trackKinds:
      - Deployment
      - StatefulSet
    skipKinds:
      - ConfigMap
      - Secret
```

## Implementation Details

The kubedog client configuration uses:
- `k8s.io/client-go` for Kubernetes API communication
- Custom rate limiting via `rest.Config.QPS` and `rest.Config.Burst`
- Separate client cache per unique (kubeContext, kubeconfig, QPS, Burst) combination

The default values (QPS=100, Burst=200) were chosen to:
- Prevent rate limiting errors in most common scenarios
- Support tracking of multiple resource types simultaneously
- Allow reasonable burst capacity for initial resource discovery
- Balance between tracking speed and API server load

## See Also

- [Issue #2445](https://github.com/helmfile/helmfile/issues/2445) - Original issue that led to configurable QPS/Burst
- [Kubedog Documentation](https://github.com/werf/kubedog)
- [Kubernetes client-go `rest.Config` QPS/Burst](https://pkg.go.dev/k8s.io/client-go/rest#Config)
