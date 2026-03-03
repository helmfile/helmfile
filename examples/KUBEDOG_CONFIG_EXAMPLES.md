# Example: Kubedog Resource Tracking Configuration

This example demonstrates various ways to configure kubedog resource tracking.

## Basic Example

```yaml
releases:
  - name: simple-app
    namespace: default
    chart: ./charts/simple-app
    trackMode: kubedog
```

Uses default QPS (100) and Burst (200).

## Customized Rate Limiting

```yaml
releases:
  - name: high-throughput-app
    namespace: production
    chart: ./charts/app
    trackMode: kubedog
    # Increased limits for large-scale deployments
    kubedogQPS: 200
    kubedogBurst: 400
    trackTimeout: 600
    trackLogs: true
    trackKinds:
      - Deployment
      - StatefulSet
```

## Multiple Releases with Different Settings

```yaml
releases:
  # Small app - conservative limits
  - name: frontend
    namespace: web
    chart: ./charts/frontend
    trackMode: kubedog
    kubedogQPS: 50
    kubedogBurst: 100
    
  # Medium app - default limits
  - name: backend
    namespace: api
    chart: ./charts/backend
    trackMode: kubedog
    
  # Large app - increased limits
  - name: data-processor
    namespace: data
    chart: ./charts/processor
    trackMode: kubedog
    kubedogQPS: 150
    kubedogBurst: 300
    trackKinds:
      - Deployment
      - StatefulSet
      - Job
```

## Environment-Specific Configuration

```yaml
environments:
  development:
    values:
      - kubedogQPS: 50
      - kubedogBurst: 100
  staging:
    values:
      - kubedogQPS: 100
      - kubedogBurst: 200
  production:
    values:
      - kubedogQPS: 200
      - kubedogBurst: 400

releases:
  - name: myapp
    namespace: {{ .Environment.Name }}
    chart: ./charts/myapp
    trackMode: kubedog
    kubedogQPS: {{ .Values.kubedogQPS }}
    kubedogBurst: {{ .Values.kubedogBurst }}
```

## With Global Defaults

```yaml
helmDefaults:
  createNamespace: true
  timeout: 300
  
releases:
  - name: app1
    namespace: default
    chart: ./charts/app
    trackMode: kubedog
    # Uses release-specific settings
    kubedogQPS: 150
    kubedogBurst: 300
    
  - name: app2
    namespace: default
    chart: ./charts/app
    trackMode: kubedog
    # Uses default QPS=100, Burst=200
```

## Selective Tracking

```yaml
releases:
  - name: complex-app
    namespace: default
    chart: ./charts/complex-app
    trackMode: kubedog
    kubedogQPS: 120
    kubedogBurst: 250
    # Only track deployments and jobs
    trackKinds:
      - Deployment
      - Job
    # Skip these resource types
    skipKinds:
      - ConfigMap
      - Secret
      - Ingress
    # Track specific resources only
    trackResources:
      - kind: Deployment
        name: main-app
      - kind: Job
        name: migration-job
        namespace: default
```

## Testing the Configuration

To test your kubedog configuration:

```bash
# Apply with kubedog tracking
helmfile apply -n my-namespace -l app=myapp

# With debug logging
helmfile apply -n my-namespace -l app=myapp --log-level debug

# With specific environment
helmfile apply -e production -l app=myapp
```

## Expected Output

When kubedog tracking is working correctly, you should see:

```
Tracking 5 resources from release myapp with kubedog
Tracking 5 resources with kubedog (filtered from 5 total)
┌ Status progress
│ DEPLOYMENT              REPLICAS   AVAILABLE   UP-TO-DATE
│ myapp-main              1/1        1           1
└ Status progress
All resources tracked successfully
UPDATED RELEASES:
NAME        NAMESPACE   CHART             VERSION   DURATION
myapp       default     ./charts/app      1.0.0     1m32s
```

## Troubleshooting Commands

```bash
# Check current kubedog settings
helmfile build -n my-namespace -l app=myapp | grep -A 5 "kubedog"

# Test with increased verbosity
helmfile apply -n my-namespace -l app=myapp --log-level debug 2>&1 | grep -i kubedog

# Monitor API server requests (requires cluster access)
kubectl get --raw /metrics | grep apiserver_request_count
```
