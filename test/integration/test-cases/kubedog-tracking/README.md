# Kubedog Integration Test

This test validates the kubedog resource tracking integration with Helmfile.

## What it tests

1. **Basic kubedog tracking**: Deploys httpbin with `trackMode: kubedog` enabled
2. **Whitelist filtering**: Uses `trackKinds` to only track Deployment resources
3. **Specific resource tracking**: Uses `trackResources` to track specific resources by name
4. **CLI flags usage**: Tests `--track-mode` and `--track-timeout` flags
5. **Cleanup**: Ensures all releases are properly deleted

## Prerequisites

- Kubernetes cluster (minikube for local testing)
- Helm 3.x installed
- kubedog library integrated (built into Helmfile)
- kubectl configured to access the cluster

## Test Cases

### httpbin-basic
- Simple deployment with kubedog tracking enabled
- `trackMode: kubedog`
- `trackTimeout: 60` seconds
- `trackLogs: false`

### httpbin-with-whitelist
- Deployment with resource kind whitelist
- Only tracks `Deployment` resources
- Skips `ConfigMap` and `Secret` resources

### httpbin-with-resources
- Deployment with specific resource tracking
- Tracks only the deployment by name and kind

## Running the test

```bash
# Run all integration tests including kubedog
./test/integration/run.sh

# Run only kubedog test (if supported by your test framework)
# Note: Currently all tests run together via run.sh
```

## Expected behavior

1. All three httpbin deployments should be created successfully
2. Kubedog should track the resources during deployment
3. Deployments should reach ready state
4. All releases should be cleaned up after tests
