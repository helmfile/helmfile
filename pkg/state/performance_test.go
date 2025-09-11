package state

import (
	"testing"

	"github.com/helmfile/helmfile/pkg/filesystem"
)

// BenchmarkDeepCopyValues ensures our fix doesn't introduce significant performance overhead
func BenchmarkDeepCopyValues(b *testing.B) {
	// Create a realistic values structure
	values := map[string]any{
		"global": map[string]any{
			"registry": "docker.io",
			"tag":      "v1.0.0",
		},
		"app1": map[string]any{
			"enabled":     true,
			"replicaCount": 3,
			"resources": map[string]any{
				"requests": map[string]any{
					"cpu":    "100m",
					"memory": "128Mi",
				},
				"limits": map[string]any{
					"cpu":    "500m", 
					"memory": "512Mi",
				},
			},
		},
		"app2": map[string]any{
			"enabled":     true,
			"replicaCount": 2,
			"config": map[string]any{
				"database": map[string]any{
					"host":     "localhost",
					"port":     5432,
					"username": "user",
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := deepCopyValues(values)
		if err != nil {
			b.Fatalf("deepCopyValues failed: %v", err)
		}
	}
}

// BenchmarkCreateReleaseTemplateData benchmarks the full template data creation process
func BenchmarkCreateReleaseTemplateData(b *testing.B) {
	fs := &filesystem.FileSystem{
		Glob: func(pattern string) ([]string, error) {
			return nil, nil
		},
		ReadFile: func(filename string) ([]byte, error) {
			return nil, nil
		},
	}

	values := map[string]any{
		"global": map[string]any{
			"registry": "docker.io",
			"tag":      "v1.0.0",
		},
		"app1": map[string]any{
			"enabled":     true,
			"replicaCount": 3,
		},
	}

	st := &HelmState{
		fs:             fs,
		basePath:       "/tmp",
		RenderedValues: values,
	}

	release := &ReleaseSpec{
		Name:  "test-app",
		Chart: "charts/test-app",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := st.createReleaseTemplateData(release, values)
		if err != nil {
			b.Fatalf("createReleaseTemplateData failed: %v", err)
		}
	}
}