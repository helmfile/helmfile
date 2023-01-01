package helmexec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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
