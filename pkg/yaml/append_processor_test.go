package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendProcessor_ProcessMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
		wantErr  bool
	}{
		{
			name: "simple append to list",
			input: map[string]any{
				"values+": []any{"new-value"},
			},
			expected: map[string]any{
				"values": []any{"new-value"},
			},
		},
		{
			name: "nested append",
			input: map[string]any{
				"config": map[string]any{
					"items+": []any{"item1", "item2"},
				},
			},
			expected: map[string]any{
				"config": map[string]any{
					"items": []any{"item1", "item2"},
				},
			},
		},
		{
			name: "mixed regular and append keys",
			input: map[string]any{
				"name":    "test",
				"values+": []any{"value1"},
				"config": map[string]any{
					"enabled": true,
					"items+":  []any{"item1"},
				},
			},
			expected: map[string]any{
				"name":   "test",
				"values": []any{"value1"},
				"config": map[string]any{
					"enabled": true,
					"items":   []any{"item1"},
				},
			},
		},
		{
			name: "non-list append value",
			input: map[string]any{
				"key+": "value",
			},
			expected: map[string]any{
				"key": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewAppendProcessor()
			result, err := processor.ProcessMap(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAppendProcessor_MergeWithAppend(t *testing.T) {
	tests := []struct {
		name     string
		dest     map[string]any
		src      map[string]any
		expected map[string]any
		wantErr  bool
	}{
		{
			name: "append to existing list",
			dest: map[string]any{
				"values": []any{"existing"},
			},
			src: map[string]any{
				"values+": []any{"new"},
			},
			expected: map[string]any{
				"values": []any{"existing", "new"},
			},
		},
		{
			name: "append to non-existent list",
			dest: map[string]any{
				"other": "value",
			},
			src: map[string]any{
				"values+": []any{"new"},
			},
			expected: map[string]any{
				"other":  "value",
				"values": []any{"new"},
			},
		},
		{
			name: "nested append",
			dest: map[string]any{
				"config": map[string]any{
					"items": []any{"existing"},
				},
			},
			src: map[string]any{
				"config": map[string]any{
					"items+": []any{"new"},
				},
			},
			expected: map[string]any{
				"config": map[string]any{
					"items": []any{"existing", "new"},
				},
			},
		},
		{
			name: "scalar with key+ treated as regular key (replace)",
			dest: map[string]any{
				"replicas": 2,
			},
			src: map[string]any{
				"replicas+": 1,
			},
			expected: map[string]any{
				"replicas": 1,
			},
		},
		{
			name: "map with key+ treated as regular key (replace)",
			dest: map[string]any{
				"resources": map[string]any{
					"limits": map[string]any{
						"memory": "256Mi",
						"cpu":    "200m",
					},
				},
			},
			src: map[string]any{
				"resources+": map[string]any{
					"requests": map[string]any{
						"memory": "128Mi",
						"cpu":    "100m",
					},
				},
			},
			expected: map[string]any{
				"resources": map[string]any{
					"requests": map[string]any{
						"memory": "128Mi",
						"cpu":    "100m",
					},
				},
			},
		},
		{
			name: "complex nested merge with key+ syntax for lists only",
			dest: map[string]any{
				"replicas": 2,
				"resources": map[string]any{
					"limits": map[string]any{
						"memory": "256Mi",
						"cpu":    "200m",
					},
					"requests": map[string]any{
						"memory": "128Mi",
						"cpu":    "100m",
					},
				},
				"service": map[string]any{
					"type": "ClusterIP",
					"port": 80,
				},
				"kube-state-metrics": map[string]any{
					"prometheus": map[string]any{
						"metricsRelabel": []any{
							map[string]any{"action": "drop"},
						},
					},
				},
			},
			src: map[string]any{
				"replicas+": 1,
				"resources+": map[string]any{
					"limits": map[string]any{
						"memory": "512Mi",
						"cpu":    "500m",
					},
					"requests": map[string]any{
						"memory": "256Mi",
						"cpu":    "250m",
					},
				},
				"service+": map[string]any{
					"type": "LoadBalancer",
					"port": 443,
					"annotations": map[string]any{
						"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
					},
				},
				"kube-state-metrics": map[string]any{
					"prometheus": map[string]any{
						"metricsRelabel+": []any{
							map[string]any{"action": "keep"},
						},
					},
				},
			},
			expected: map[string]any{
				"replicas": 1,
				"resources": map[string]any{
					"limits": map[string]any{
						"memory": "512Mi",
						"cpu":    "500m",
					},
					"requests": map[string]any{
						"memory": "256Mi",
						"cpu":    "250m",
					},
				},
				"service": map[string]any{
					"type": "LoadBalancer",
					"port": 443,
					"annotations": map[string]any{
						"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
					},
				},
				"kube-state-metrics": map[string]any{
					"prometheus": map[string]any{
						"metricsRelabel": []any{
							map[string]any{"action": "drop"},
							map[string]any{"action": "keep"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewAppendProcessor()
			err := processor.MergeWithAppend(tt.dest, tt.src)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, tt.dest)
		})
	}
}

func TestUnmarshalWithAppend(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		expected map[string]any
		wantErr  bool
	}{
		{
			name: "simple append syntax",
			yamlData: `
values+:
  - item1
  - item2
name: test
`,
			expected: map[string]any{
				"values": []any{"item1", "item2"},
				"name":   "test",
			},
		},
		{
			name: "nested append syntax",
			yamlData: `
config:
  items+:
    - existing
    - new
  enabled: true
`,
			expected: map[string]any{
				"config": map[string]any{
					"items":   []any{"existing", "new"},
					"enabled": true,
				},
			},
		},
		{
			name: "complex values file with key+ syntax",
			yamlData: `
replicas+: 1
resources+:
  limits:
    memory: 512Mi
    cpu: 500m
  requests:
    memory: 256Mi
    cpu: 250m
service+:
  type: LoadBalancer
  port: 443
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: nlb
kube-state-metrics:
  prometheus:
    metricsRelabel+:
      - action: keep
`,
			expected: map[string]any{
				"replicas": 1,
				"resources": map[string]any{
					"limits": map[string]any{
						"memory": "512Mi",
						"cpu":    "500m",
					},
					"requests": map[string]any{
						"memory": "256Mi",
						"cpu":    "250m",
					},
				},
				"service": map[string]any{
					"type": "LoadBalancer",
					"port": 443,
					"annotations": map[string]any{
						"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
					},
				},
				"kube-state-metrics": map[string]any{
					"prometheus": map[string]any{
						"metricsRelabel": []any{
							map[string]any{"action": "keep"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]any
			err := UnmarshalWithAppend([]byte(tt.yamlData), &result)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsAppendKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"key+", true},
		{"key", false},
		{"key++", true},
		{"+key", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := IsAppendKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetBaseKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"key+", "key"},
		{"key", "key"},
		{"key++", "key+"},
		{"+key", "+key"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := GetBaseKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAppendProcessor_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]any
		wantErr bool
	}{
		{
			name: "invalid map with non-string key",
			input: map[string]any{
				"valid": map[any]any{
					123: "invalid", // non-string key
				},
			},
			wantErr: true,
		},
		{
			name: "valid map with string keys",
			input: map[string]any{
				"valid": map[string]any{
					"key": "value",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := NewAppendProcessor()
			_, err := processor.ProcessMap(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnmarshalWithAppend_ErrorCases(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		wantErr  bool
	}{
		{
			name: "invalid YAML",
			yamlData: `
invalid: yaml: content
  - missing: proper: structure
`,
			wantErr: true,
		},
		{
			name: "valid YAML with key+",
			yamlData: `
valid: true
key+: value
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]any
			err := UnmarshalWithAppend([]byte(tt.yamlData), &result)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
