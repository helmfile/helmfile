package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const testManifest = `---
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
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
data:
  key: value
`

const emptyManifest = `---
# Empty manifest
`

const malformedManifest = `
apiVersion: v1
kind: Service
metadata:
  name: my-service
  namespace: default
spec:
  invalid: [unclosed
`

func TestGetReleaseResourcesFromManifest(t *testing.T) {
	tests := []struct {
		name             string
		manifest         []byte
		releaseName      string
		releaseNamespace string
		expectedCount    int
		expectedKinds    []string
		wantErr          bool
	}{
		{
			name:             "valid manifest",
			manifest:         []byte(testManifest),
			releaseName:      "my-release",
			releaseNamespace: "default",
			expectedCount:    3,
			expectedKinds:    []string{"Service", "Deployment", "ConfigMap"},
			wantErr:          false,
		},
		{
			name:             "empty manifest",
			manifest:         []byte(emptyManifest),
			releaseName:      "my-release",
			releaseNamespace: "default",
			expectedCount:    0,
			expectedKinds:    []string{},
			wantErr:          false,
		},
		{
			name:             "nil manifest",
			manifest:         nil,
			releaseName:      "my-release",
			releaseNamespace: "default",
			expectedCount:    0,
			expectedKinds:    []string{},
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources, err := GetReleaseResourcesFromManifest(tt.manifest, tt.releaseName, tt.releaseNamespace)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resources)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resources)

			assert.Equal(t, tt.releaseName, resources.ReleaseName)
			assert.Equal(t, tt.releaseNamespace, resources.Namespace)
			assert.Len(t, resources.Resources, tt.expectedCount)

			if tt.expectedKinds != nil {
				actualKinds := make([]string, len(resources.Resources))
				for i, res := range resources.Resources {
					actualKinds[i] = res.Kind
				}
				assert.ElementsMatch(t, tt.expectedKinds, actualKinds)
			}
		})
	}
}

func TestGetReleaseResourcesFromManifestWithLogger(t *testing.T) {
	logger := zap.NewNop().Sugar()
	resources, err := GetReleaseResourcesFromManifestWithLogger(logger, []byte(testManifest), "my-release", "default", nil)

	require.NoError(t, err)
	require.NotNil(t, resources)

	assert.Equal(t, "my-release", resources.ReleaseName)
	assert.Equal(t, "default", resources.Namespace)
	assert.Len(t, resources.Resources, 3)

	assert.Contains(t, []string{"Service", "Deployment", "ConfigMap"}, resources.Resources[0].Kind)
}

func TestIsTrackableKind(t *testing.T) {
	tests := []struct {
		kind     string
		expected bool
	}{
		{"Deployment", true},
		{"StatefulSet", true},
		{"DaemonSet", true},
		{"Job", true},
		{"Pod", true},
		{"ReplicaSet", true},
		{"Service", false},
		{"ConfigMap", false},
		{"Secret", false},
		{"Ingress", false},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			result := IsTrackableKind(tt.kind)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsStaticKind(t *testing.T) {
	tests := []struct {
		kind     string
		expected bool
	}{
		{"Service", true},
		{"ConfigMap", true},
		{"Secret", true},
		{"PersistentVolumeClaim", true},
		{"Ingress", true},
		{"Deployment", false},
		{"StatefulSet", false},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			result := IsStaticKind(tt.kind)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetHelmReleaseLabels(t *testing.T) {
	labels := GetHelmReleaseLabels("my-release", "my-namespace")

	expectedLabels := map[string]string{
		"meta.helm.sh/release-name":      "my-release",
		"meta.helm.sh/release-namespace": "my-namespace",
	}

	assert.Equal(t, expectedLabels, labels)
}

func TestGetHelmReleaseAnnotations(t *testing.T) {
	annotations := GetHelmReleaseAnnotations("my-release")

	expectedAnnotations := map[string]string{
		"meta.helm.sh/release-name": "my-release",
	}

	assert.Equal(t, expectedAnnotations, annotations)
}

func TestParseManifest(t *testing.T) {
	resources, err := parseManifest([]byte(testManifest), nil)

	require.NoError(t, err)
	assert.Len(t, resources, 3)

	assert.Equal(t, "Service", resources[0].Kind)
	assert.Equal(t, "my-service", resources[0].Name)
	assert.Equal(t, "default", resources[0].Namespace)

	assert.Equal(t, "Deployment", resources[1].Kind)
	assert.Equal(t, "my-deployment", resources[1].Name)

	assert.Equal(t, "ConfigMap", resources[2].Kind)
	assert.Equal(t, "my-config", resources[2].Name)
}

func TestParseManifest_Empty(t *testing.T) {
	resources, err := parseManifest([]byte(emptyManifest), nil)

	require.NoError(t, err)
	assert.Empty(t, resources)
}

func TestParseManifest_Nil(t *testing.T) {
	resources, err := parseManifest(nil, nil)

	require.NoError(t, err)
	assert.Empty(t, resources)
}

func TestResource_ManifestContent(t *testing.T) {
	resources, err := parseManifest([]byte(testManifest), nil)

	require.NoError(t, err)
	require.Len(t, resources, 3)

	for _, res := range resources {
		assert.NotEmpty(t, res.Manifest)
		assert.Contains(t, res.Manifest, "apiVersion")
		assert.Contains(t, res.Manifest, "kind")
	}
}

func TestTrackConfig_TrackKinds(t *testing.T) {
	config := &TrackConfig{
		TrackKinds: []string{"Deployment"},
	}

	resources, err := GetReleaseResourcesFromManifestWithConfig(
		[]byte(testManifest),
		"test-release",
		"default",
		config,
	)

	require.NoError(t, err)
	require.NotNil(t, resources)

	assert.Len(t, resources.Resources, 1)
	assert.Equal(t, "Deployment", resources.Resources[0].Kind)
	assert.Equal(t, "my-deployment", resources.Resources[0].Name)
}

func TestTrackConfig_SkipKinds(t *testing.T) {
	config := &TrackConfig{
		SkipKinds: []string{"ConfigMap"},
	}

	resources, err := GetReleaseResourcesFromManifestWithConfig(
		[]byte(testManifest),
		"test-release",
		"default",
		config,
	)

	require.NoError(t, err)
	require.NotNil(t, resources)

	assert.Len(t, resources.Resources, 2)

	kinds := make([]string, len(resources.Resources))
	for i, res := range resources.Resources {
		kinds[i] = res.Kind
	}
	assert.NotContains(t, kinds, "ConfigMap")
}

func TestTrackConfig_CustomTrackableKinds(t *testing.T) {
	config := &TrackConfig{
		CustomTrackableKinds: []string{"CronJob"},
	}

	isTrackable := IsTrackableKindWithConfig("CronJob", config)
	assert.True(t, isTrackable)

	isNotTrackable := IsTrackableKindWithConfig("Deployment", config)
	assert.False(t, isNotTrackable)
}

func TestTrackConfig_CustomStaticKinds(t *testing.T) {
	config := &TrackConfig{
		CustomStaticKinds: []string{"CustomResource"},
	}

	isStatic := IsStaticKindWithConfig("CustomResource", config)
	assert.True(t, isStatic)

	isNotStatic := IsStaticKindWithConfig("ConfigMap", config)
	assert.False(t, isNotStatic)
}

func TestTrackConfig_Combined(t *testing.T) {
	config := &TrackConfig{
		SkipKinds:            []string{"ConfigMap"},
		CustomTrackableKinds: []string{"CronJob"},
		CustomStaticKinds:    []string{"CustomResource"},
	}

	trackable1 := IsTrackableKindWithConfig("CronJob", config)
	assert.True(t, trackable1)

	defaultTrackable := IsTrackableKindWithConfig("Service", config)
	assert.False(t, defaultTrackable, "Service is default trackable, but CustomTrackableKinds is configured, so it should not be trackable")

	static1 := IsStaticKindWithConfig("CustomResource", config)
	assert.True(t, static1)

	defaultStatic := IsStaticKindWithConfig("ConfigMap", config)
	assert.False(t, defaultStatic, "ConfigMap is default static, but CustomStaticKinds is configured, so it should not be static")
}
