package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	chart "helm.sh/helm/v4/pkg/chart/v2"

	"github.com/helmfile/helmfile/pkg/helmexec"
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

			// Create a mock helm interface
			helm := &mockHelmExec{version: tt.helmVersion}

			// Call the function
			qualifiedChartName, chartName, chartVersion, err := st.getOCIQualifiedChartName(release, helm)

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

// mockHelmExec is a minimal mock implementation of helmexec.Interface for testing
type mockHelmExec struct {
	version string
}

func (m *mockHelmExec) IsVersionAtLeast(version string) bool {
	// Parse both versions for proper comparison
	// Expected format: "major.minor.patch"
	parsedMock := parseSimpleVersion(m.version)
	parsedCheck := parseSimpleVersion(version)

	if parsedMock[0] != parsedCheck[0] {
		return parsedMock[0] >= parsedCheck[0]
	}
	if parsedMock[1] != parsedCheck[1] {
		return parsedMock[1] >= parsedCheck[1]
	}
	return parsedMock[2] >= parsedCheck[2]
}

// parseSimpleVersion parses a version string like "3.18.0" into [major, minor, patch]
func parseSimpleVersion(v string) [3]int {
	parts := [3]int{0, 0, 0}
	var nums []int
	current := 0
	for _, ch := range v {
		if ch >= '0' && ch <= '9' {
			current = current*10 + int(ch-'0')
		} else if ch == '.' {
			nums = append(nums, current)
			current = 0
		}
	}
	nums = append(nums, current)

	for i := 0; i < len(nums) && i < 3; i++ {
		parts[i] = nums[i]
	}
	return parts
}

// Implement other required methods as no-ops to match helmexec.Interface
func (m *mockHelmExec) SetExtraArgs(...string)     {}
func (m *mockHelmExec) SetHelmBinary(string)       {}
func (m *mockHelmExec) SetEnableLiveOutput(bool)   {}
func (m *mockHelmExec) SetDisableForceUpdate(bool) {}
func (m *mockHelmExec) AddRepo(string, string, string, string, string, string, string, string, bool, bool) error {
	return nil
}
func (m *mockHelmExec) UpdateRepo() error { return nil }
func (m *mockHelmExec) RegistryLogin(string, string, string, string, string, string, bool) error {
	return nil
}
func (m *mockHelmExec) BuildDeps(string, string, ...string) error { return nil }
func (m *mockHelmExec) UpdateDeps(string) error                   { return nil }
func (m *mockHelmExec) SyncRelease(helmexec.HelmContext, string, string, string, ...string) error {
	return nil
}
func (m *mockHelmExec) DiffRelease(helmexec.HelmContext, string, string, string, bool, ...string) error {
	return nil
}
func (m *mockHelmExec) TemplateRelease(string, string, ...string) error             { return nil }
func (m *mockHelmExec) Fetch(string, ...string) error                               { return nil }
func (m *mockHelmExec) ChartPull(string, string, ...string) error                   { return nil }
func (m *mockHelmExec) ChartExport(string, string) error                            { return nil }
func (m *mockHelmExec) Lint(string, string, ...string) error                        { return nil }
func (m *mockHelmExec) ReleaseStatus(helmexec.HelmContext, string, ...string) error { return nil }
func (m *mockHelmExec) DeleteRelease(helmexec.HelmContext, string, ...string) error { return nil }
func (m *mockHelmExec) TestRelease(helmexec.HelmContext, string, ...string) error   { return nil }
func (m *mockHelmExec) List(helmexec.HelmContext, string, ...string) (string, error) {
	return "", nil
}
func (m *mockHelmExec) DecryptSecret(helmexec.HelmContext, string, ...string) (string, error) {
	return "", nil
}
func (m *mockHelmExec) IsHelm3() bool { return true }
func (m *mockHelmExec) IsHelm4() bool { return false }
func (m *mockHelmExec) GetVersion() helmexec.Version {
	// Parse the version string into a Version struct
	// This is a simplified parser for testing
	major, minor, patch := 3, 18, 0
	if m.version == "3.7.0" {
		major, minor, patch = 3, 7, 0
	}
	return helmexec.Version{Major: major, Minor: minor, Patch: patch}
}
func (m *mockHelmExec) ShowChart(string) (chart.Metadata, error) {
	return chart.Metadata{}, nil
}

// IsOCIChart is a helper function to check if a chart is OCI-based
func IsOCIChart(chart string) bool {
	return len(chart) > 6 && chart[:6] == "oci://"
}
