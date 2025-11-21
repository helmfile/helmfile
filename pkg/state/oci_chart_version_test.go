package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOCIChartVersionHandling tests the handling of OCI chart versions (issue #2247)
func TestOCIChartVersionHandling(t *testing.T) {
	tests := []struct {
		name                   string
		chart                  string
		version                string
		devel                  bool
		helmVersion            string
		expectedVersion        string
		expectedError          bool
		expectedQualifiedChart string
	}{
		{
			name:                   "OCI chart with explicit version",
			chart:                  "oci://registry.example.com/my-chart",
			version:                "1.2.3",
			helmVersion:            "3.18.0",
			expectedVersion:        "1.2.3",
			expectedError:          false,
			expectedQualifiedChart: "registry.example.com/my-chart:1.2.3",
		},
		{
			name:                   "OCI chart with semver range version",
			chart:                  "oci://registry.example.com/my-chart",
			version:                "^1.0.0",
			helmVersion:            "3.18.0",
			expectedVersion:        "^1.0.0",
			expectedError:          false,
			expectedQualifiedChart: "registry.example.com/my-chart:^1.0.0",
		},
		{
			name:                   "OCI chart without version should use empty string",
			chart:                  "oci://registry.example.com/my-chart",
			version:                "",
			helmVersion:            "3.18.0",
			expectedVersion:        "",
			expectedError:          false,
			expectedQualifiedChart: "registry.example.com/my-chart",
		},
		{
			name:                   "OCI chart with explicit 'latest' should fail (any Helm version)",
			chart:                  "oci://registry.example.com/my-chart",
			version:                "latest",
			helmVersion:            "3.18.0",
			expectedVersion:        "",
			expectedError:          true,
			expectedQualifiedChart: "",
		},
		{
			name:                   "OCI chart with explicit 'latest' should also fail on older Helm",
			chart:                  "oci://registry.example.com/my-chart",
			version:                "latest",
			helmVersion:            "3.7.0",
			expectedVersion:        "",
			expectedError:          true,
			expectedQualifiedChart: "",
		},
		{
			name:                   "OCI chart without version in devel mode",
			chart:                  "oci://registry.example.com/my-chart",
			version:                "",
			devel:                  true,
			helmVersion:            "3.18.0",
			expectedVersion:        "",
			expectedError:          false,
			expectedQualifiedChart: "registry.example.com/my-chart",
		},
		{
			name:                   "non-OCI chart returns empty qualified name",
			chart:                  "stable/nginx",
			version:                "",
			helmVersion:            "3.18.0",
			expectedVersion:        "",
			expectedError:          false,
			expectedQualifiedChart: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal HelmState
			st := &HelmState{
				basePath: "/test",
			}

			// Create a release
			release := &ReleaseSpec{
				Name:    "test-release",
				Chart:   tt.chart,
				Version: tt.version,
			}

			if tt.devel {
				devel := true
				release.Devel = &devel
			}

			// Call the function
			qualifiedChartName, chartName, chartVersion, err := st.getOCIQualifiedChartName(release)

			// Check error
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "semver compliant")
			} else {
				require.NoError(t, err)
			}

			// Check version
			assert.Equal(t, tt.expectedVersion, chartVersion, "chartVersion mismatch")

			// Check qualified chart name
			assert.Equal(t, tt.expectedQualifiedChart, qualifiedChartName, "qualifiedChartName mismatch")

			// Check chart name extraction for OCI charts
			if IsOCIChart(tt.chart) && !tt.expectedError {
				assert.Equal(t, "my-chart", chartName, "chartName mismatch")
			}
		})
	}
}

// IsOCIChart is a helper function to check if a chart is OCI-based
func IsOCIChart(chart string) bool {
	return len(chart) > 6 && chart[:6] == "oci://"
}
