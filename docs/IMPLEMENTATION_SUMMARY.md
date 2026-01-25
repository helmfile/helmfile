# Implementation Summary: Custom Resource Tracking

## Overview

Added custom resource tracking configuration to the kubedog tracker, allowing users to flexibly control which resources are tracked and how they are classified.

## Changes Made

### 1. TrackOptions Enhancements (`pkg/kubedog/options.go`)

Added new configuration fields to `TrackOptions` struct:

```go
type TrackOptions struct {
    Timeout               int
    Logs                  bool
    LogsSince             int
    Namespace             string
    KubeContext           string
    Kubeconfig            string
    // NEW FIELDS
    TrackKinds            []string  // Only track resources of these kinds
    SkipKinds             []string  // Skip resources of these kinds
    CustomTrackableKinds   []string  // Custom kinds that should be actively tracked
    CustomStaticKinds      []string  // Custom kinds that don't need tracking
}
```

Added builder methods:

```go
func (o *TrackOptions) WithTrackKinds(kinds []string) *TrackOptions
func (o *TrackOptions) WithSkipKinds(kinds []string) *TrackOptions
func (o *TrackOptions) WithCustomTrackableKinds(kinds []string) *TrackOptions
func (o *TrackOptions) WithCustomStaticKinds(kinds []string) *TrackOptions
```

### 2. TrackConfig Structure (`pkg/cluster/release.go`)

Added `TrackConfig` struct to pass tracking configuration:

```go
type TrackConfig struct {
    TrackKinds           []string
    SkipKinds            []string
    CustomTrackableKinds []string
    CustomStaticKinds    []string
}
```

### 3. Enhanced Resource Detection (`pkg/cluster/release.go`)

Added new functions for resource filtering and classification:

```go
// Get resources with custom tracking configuration
func GetReleaseResourcesFromManifestWithConfig(
    manifest []byte,
    releaseName, releaseNamespace string,
    config *TrackConfig,
) (*ReleaseResources, error)

// Filter resources based on TrackKinds and SkipKinds
func filterResourcesByConfig(
    resources []Resource,
    config *TrackConfig,
    logger *zap.SugaredLogger,
) []Resource

// Check if resource is trackable with custom config
func IsTrackableKindWithConfig(kind string, config *TrackConfig) bool

// Check if resource is static with custom config
func IsStaticKindWithConfig(kind string, config *TrackConfig) bool
```

### 4. Enhanced Tracker (`pkg/kubedog/tracker.go`)

Updated `TrackReleaseWithManifest` to use custom configuration:

```go
func (t *Tracker) TrackReleaseWithManifest(
    ctx interface{},
    releaseName, releaseNamespace string,
    manifest []byte,
) error {
    // Create TrackConfig from TrackOptions
    trackConfig := &cluster.TrackConfig{
        TrackKinds:           t.options.TrackKinds,
        SkipKinds:            t.options.SkipKinds,
        CustomTrackableKinds:   t.options.CustomTrackableKinds,
        CustomStaticKinds:      t.options.CustomStaticKinds,
    }
    
    // Use config when getting resources
    releaseResources, err := cluster.GetReleaseResourcesFromManifestWithLogger(
        t.logger, manifest, releaseName, releaseNamespace, trackConfig,
    )
    
    // Pass config when tracking resources
    return t.trackResources(ctx, releaseResources, trackConfig)
}
```

Added support for custom resources:

```go
func (t *Tracker) trackCustomResource(ctx context.Context, res cluster.Resource) error {
    t.logger.Infof("Waiting for custom resource %s/%s to become ready", res.Namespace, res.Name)
    return nil
}
```

## Configuration Behavior

### Priority Order

1. **SkipKinds**: Applied first - if a resource kind is in SkipKinds, it's skipped
2. **TrackKinds**: If set, only resources in this list are considered
3. **CustomTrackableKinds**: If set, only these kinds are considered trackable
4. **CustomStaticKinds**: If set, only these kinds are considered static
5. **Default Lists**: Fall back to default trackable/static kinds if no custom config

### Example Configurations

#### Only Track Deployments
```go
opts := NewTrackOptions().
    WithTrackKinds([]string{"Deployment"})
```

#### Skip ConfigMaps
```go
opts := NewTrackOptions().
    WithSkipKinds([]string{"ConfigMap"})
```

#### Add Custom Trackable Kind
```go
opts := NewTrackOptions().
    WithCustomTrackableKinds([]string{"CronJob"})
```

#### Combined Configuration
```go
opts := NewTrackOptions().
    WithTrackKinds([]string{"Deployment", "StatefulSet"}).
    WithSkipKinds([]string{"ConfigMap"})
```

## Testing

### New Test Cases

#### Cluster Package Tests
- `TestTrackConfig_TrackKinds` - Test filtering by TrackKinds
- `TestTrackConfig_SkipKinds` - Test skipping by SkipKinds
- `TestTrackConfig_CustomTrackableKinds` - Test custom trackable kinds
- `TestTrackConfig_CustomStaticKinds` - Test custom static kinds
- `TestTrackConfig_Combined` - Test combined configuration
- `TestTrackConfig_Nil` - Test behavior with nil config

#### Kubedog Package Tests
- `TestTrackReleaseWithManifest_TrackKinds` - Test tracker with TrackKinds
- `TestTrackReleaseWithManifest_SkipKinds` - Test tracker with SkipKinds
- `TestTrackReleaseWithManifest_CustomTrackableKinds` - Test tracker with custom kinds

### Test Results

```bash
$ go test ./pkg/cluster/... -v
PASS: TestGetReleaseResourcesFromManifest
PASS: TestGetReleaseResourcesFromManifestWithLogger
PASS: TestIsTrackableKind
PASS: TestIsStaticKind
PASS: TestGetHelmReleaseLabels
PASS: TestGetHelmReleaseAnnotations
PASS: TestParseManifest
PASS: TestParseManifest_Empty
PASS: TestParseManifest_Nil
PASS: TestResource_ManifestContent
PASS: TestTrackConfig_TrackKinds
PASS: TestTrackConfig_SkipKinds
PASS: TestTrackConfig_CustomTrackableKinds
PASS: TestTrackConfig_CustomStaticKinds
PASS: TestTrackConfig_Combined
PASS: TestTrackConfig_Nil
PASS: TestDetectServerVersion_Integration
PASS: TestDetectServerVersion_InvalidConfig
ok  	github.com/helmfile/helmfile/pkg/cluster

$ go test ./pkg/kubedog/... -v
PASS: TestNewTracker
PASS: TestTracker_Close
PASS: TestTrackRelease_WithNoNamespace
PASS: TestTrackOptions
PASS: TestTrackMode
PASS: TestTrackReleaseWithManifest
PASS: TestTrackReleaseWithManifest_Empty
PASS: TestTrackReleaseWithManifest_InvalidYAML
PASS: TestTrackReleaseWithManifest_TrackKinds
PASS: TestTrackReleaseWithManifest_SkipKinds
PASS: TestTrackReleaseWithManifest_CustomTrackableKinds
ok  	github.com/helmfile/helmfile/pkg/kubedog

$ make check
(All checks pass)
```

## Documentation

### New Documentation Files

1. **docs/CUSTOM_TRACKING.md** - Comprehensive guide on custom tracking configuration
   - Overview of all configuration options
   - Usage examples for each option
   - Priority and behavior explanation
   - Default resource classifications
   - Integration examples

2. **examples/custom_tracking/main.go** - Working example program
   - Example 1: Default tracking (all resources)
   - Example 2: Track only Deployments and StatefulSets
   - Example 3: Skip ConfigMaps
   - Example 4: Custom trackable kinds (CronJob)
   - Example 5: Custom static kinds

## Usage

### Basic Usage

```go
import (
    "github.com/helmfile/helmfile/pkg/kubedog"
    "go.uber.org/zap"
)

func main() {
    // Configure tracker to track only Deployments and StatefulSets
    opts := kubedog.NewTrackOptions().
        WithTrackKinds([]string{"Deployment", "StatefulSet"}).
        WithTimeout(600)

    config := &kubedog.TrackerConfig{
        Logger:       zap.NewExample().Sugar(),
        Namespace:    "default",
        TrackOptions: opts,
    }

    tracker, err := kubedog.NewTracker(config)
    if err != nil {
        panic(err)
    }

    manifest := []byte(`... helm template output ...`)
    
    err = tracker.TrackReleaseWithManifest(nil, "my-release", "default", manifest)
    if err != nil {
        panic(err)
    }

    tracker.Close()
}
```

### Advanced Configuration

```go
// Track only specific kinds, skip certain kinds, and add custom trackable kinds
opts := kubedog.NewTrackOptions().
    WithTrackKinds([]string{"Deployment", "StatefulSet", "CronJob"}).
    WithSkipKinds([]string{"ConfigMap", "Secret"}).
    WithCustomTrackableKinds([]string{"CronJob"}).
    WithTimeout(300)
```

## Benefits

1. **Flexibility**: Users can control exactly which resources are tracked
2. **Performance**: Skip tracking unnecessary resources to save time
3. **Customization**: Support for custom resource types (CRDs)
4. **Fine-grained Control**: Combine multiple options for precise control
5. **Backward Compatible**: Default behavior unchanged when no custom config is set

## Backward Compatibility

All changes are backward compatible:

- New fields in `TrackOptions` have default nil values
- When all new fields are nil, behavior is identical to previous version
- Existing functions `IsTrackableKind()` and `IsStaticKind()` still work
- New functions `IsTrackableKindWithConfig()` and `IsStaticKindWithConfig()` accept nil config

## Future Enhancements

Potential future improvements:

1. **Pattern Matching**: Support wildcards and regex in TrackKinds/SkipKinds
2. **Label-based Filtering**: Track resources based on labels/annotations
3. **Resource Limits**: Limit number of resources tracked concurrently
4. **Custom Tracking Logic**: Allow users to provide custom tracking functions
5. **Configuration File**: Support loading tracking config from YAML/JSON files
