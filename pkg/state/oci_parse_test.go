package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseOCIChartRef(t *testing.T) {
	tests := []struct {
		name           string
		chartURL       string
		expectedBase   string
		expectedVer    string
		expectedDigest string
	}{
		{
			name:           "plain OCI URL",
			chartURL:       "oci://registry/chart",
			expectedBase:   "oci://registry/chart",
			expectedVer:    "",
			expectedDigest: "",
		},
		{
			name:           "OCI URL with version",
			chartURL:       "oci://registry/chart:2.0.0",
			expectedBase:   "oci://registry/chart",
			expectedVer:    "2.0.0",
			expectedDigest: "",
		},
		{
			name:           "OCI URL with digest",
			chartURL:       "oci://registry/chart@sha256:abc",
			expectedBase:   "oci://registry/chart",
			expectedVer:    "",
			expectedDigest: "sha256:abc",
		},
		{
			name:           "OCI URL with version and digest",
			chartURL:       "oci://reg/chart:2.0@sha256:abc",
			expectedBase:   "oci://reg/chart",
			expectedVer:    "2.0",
			expectedDigest: "sha256:abc",
		},
		{
			name:           "OCI URL with port, version, and digest",
			chartURL:       "oci://reg:5000/chart:1.0@sha256:a",
			expectedBase:   "oci://reg:5000/chart",
			expectedVer:    "1.0",
			expectedDigest: "sha256:a",
		},
		{
			name:           "OCI URL with port only",
			chartURL:       "oci://reg:5000/chart",
			expectedBase:   "oci://reg:5000/chart",
			expectedVer:    "",
			expectedDigest: "",
		},
		{
			name:           "OCI URL with port and digest, no version",
			chartURL:       "oci://reg:5000/chart@sha256:abc",
			expectedBase:   "oci://reg:5000/chart",
			expectedVer:    "",
			expectedDigest: "sha256:abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ver, digest := parseOCIChartRef(tt.chartURL)
			assert.Equal(t, tt.expectedBase, base, "baseURL mismatch")
			assert.Equal(t, tt.expectedVer, ver, "version mismatch")
			assert.Equal(t, tt.expectedDigest, digest, "digest mismatch")
		})
	}
}

func TestParseVersionDigest(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		expectedVer    string
		expectedDigest string
	}{
		{
			name:           "version only",
			version:        "2.0.0",
			expectedVer:    "2.0.0",
			expectedDigest: "",
		},
		{
			name:           "version with digest",
			version:        "2.0.0@sha256:abc",
			expectedVer:    "2.0.0",
			expectedDigest: "sha256:abc",
		},
		{
			name:           "digest only",
			version:        "@sha256:abc",
			expectedVer:    "",
			expectedDigest: "sha256:abc",
		},
		{
			name:           "empty string",
			version:        "",
			expectedVer:    "",
			expectedDigest: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver, digest := parseVersionDigest(tt.version)
			assert.Equal(t, tt.expectedVer, ver, "version mismatch")
			assert.Equal(t, tt.expectedDigest, digest, "digest mismatch")
		})
	}
}
