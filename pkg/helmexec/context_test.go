package helmexec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"
)

// TestGetTillerlessArgs tests the GetTillerlessArgs function
func TestGetTillerlessArgs(t *testing.T) {
	helmBinary := "helm"

	tests := []struct {
		tillerless       bool
		helmMajorVersion string
		tillerNamespace  string
		expected         []string
	}{
		{
			tillerless:       true,
			helmMajorVersion: "2.0.0",
			expected:         []string{"tiller", "run", "--", helmBinary},
		},
		{
			tillerless:       true,
			helmMajorVersion: "2.0.0",
			tillerNamespace:  "test-namespace",
			expected:         []string{"tiller", "run", "test-namespace", "--", helmBinary},
		},
		{
			tillerless:       false,
			helmMajorVersion: "2.0.0",
			expected:         []string{},
		},
		{
			tillerless:       true,
			helmMajorVersion: "3.0.0",
			expected:         []string{},
		},
	}
	for _, test := range tests {
		hc := &HelmContext{
			Tillerless:      test.tillerless,
			TillerNamespace: test.tillerNamespace,
		}
		sr, _ := semver.NewVersion(test.helmMajorVersion)
		he := &execer{
			helmBinary: helmBinary,
			version:    *sr,
		}
		require.Equalf(t, test.expected, hc.GetTillerlessArgs(he), "expected result %s, received result %s", test.expected, hc.GetTillerlessArgs(he))

	}
}

func pwd() string {
	pwd, _ := os.Getwd()
	return pwd
}

// TestGetTillerlessEnv tests the getTillerlessEnv function
func TestGetTillerlessEnv(t *testing.T) {
	kubeconfigEnv := "KUBECONFIG"

	tests := []struct {
		tillerless bool
		kubeconfig string
		expected   map[string]string
	}{
		{
			tillerless: true,
			kubeconfig: "",
			expected:   map[string]string{"HELM_TILLER_SILENT": "true"},
		},
		{
			tillerless: true,
			kubeconfig: "abc",
			expected:   map[string]string{"HELM_TILLER_SILENT": "true", kubeconfigEnv: filepath.Join(pwd(), "abc")},
		},
		{
			tillerless: true,
			kubeconfig: "/path/to/kubeconfig",
			expected:   map[string]string{"HELM_TILLER_SILENT": "true", kubeconfigEnv: "/path/to/kubeconfig"},
		},
		{
			tillerless: false,
			expected:   map[string]string{},
		},
	}
	for _, test := range tests {
		hc := &HelmContext{
			Tillerless: test.tillerless,
		}
		t.Setenv(kubeconfigEnv, test.kubeconfig)
		result := hc.getTillerlessEnv()
		require.Equalf(t, test.expected, result, "expected result %s, received result %s", test.expected, result)

	}
}
