package plugins

import (
	"io"
	"testing"

	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/envvar"
)

func TestValsInstance(t *testing.T) {
	i, err := ValsInstance()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	i2, _ := ValsInstance()

	if i != i2 {
		t.Error("Instances should be equal")
	}
}

func TestBuildValsOptions(t *testing.T) {
	tests := []struct {
		name                       string
		awsLogLevel                string
		failOnMissingKey           string
		expectedLogLevel           string
		expectedFailOnMissingKey   bool
		expectedLogOutputDiscarded bool
		expectError                bool
	}{
		{
			name:                       "defaults",
			awsLogLevel:                "",
			failOnMissingKey:           "",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "explicit failOnMissingKey true",
			awsLogLevel:                "",
			failOnMissingKey:           "true",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   true,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "failOnMissingKey false",
			awsLogLevel:                "",
			failOnMissingKey:           "false",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "failOnMissingKey with whitespace",
			awsLogLevel:                "",
			failOnMissingKey:           "  true  ",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   true,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "failOnMissingKey uppercase TRUE",
			awsLogLevel:                "",
			failOnMissingKey:           "TRUE",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   true,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "failOnMissingKey numeric 1",
			awsLogLevel:                "",
			failOnMissingKey:           "1",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   true,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "failOnMissingKey numeric 0",
			awsLogLevel:                "",
			failOnMissingKey:           "0",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: true,
		},
		{
			name:             "failOnMissingKey invalid value",
			awsLogLevel:      "",
			failOnMissingKey: "invalid",
			expectError:      true,
		},
		{
			name:                       "aws log level verbose",
			awsLogLevel:                "verbose",
			failOnMissingKey:           "",
			expectedLogLevel:           "verbose",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: false,
		},
		{
			name:                       "aws log level with whitespace",
			awsLogLevel:                "  minimal  ",
			failOnMissingKey:           "",
			expectedLogLevel:           "minimal",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: false,
		},
		{
			name:                       "aws log level OFF uppercase",
			awsLogLevel:                "OFF",
			failOnMissingKey:           "",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "aws log level Off mixed case",
			awsLogLevel:                "Off",
			failOnMissingKey:           "",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "aws log level Off mixed case",
			awsLogLevel:                "Off",
			failOnMissingKey:           "",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "both options set",
			awsLogLevel:                "standard",
			failOnMissingKey:           "true",
			expectedLogLevel:           "standard",
			expectedFailOnMissingKey:   true,
			expectedLogOutputDiscarded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envvar.AWSSDKLogLevel, tt.awsLogLevel)
			t.Setenv(envvar.ValsFailOnMissingKeyInMap, tt.failOnMissingKey)

			opts, err := buildValsOptions()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), envvar.ValsFailOnMissingKeyInMap)
				return
			}

			require.NoError(t, err)

			assert.Equal(t, tt.expectedLogLevel, opts.AWSLogLevel)
			assert.Equal(t, tt.expectedFailOnMissingKey, opts.FailOnMissingKeyInMap)
			assert.Equal(t, valsCacheSize, opts.CacheSize)

			isDiscarded := opts.LogOutput == io.Discard
			assert.Equal(t, tt.expectedLogOutputDiscarded, isDiscarded)
		})
	}
}

func TestAWSSDKLogLevelConfiguration(t *testing.T) {
	tests := []struct {
		name              string
		envValue          string
		expectedLogLevel  string
		expectedLogOutput bool
	}{
		{
			name:              "no env var defaults to off",
			envValue:          "",
			expectedLogLevel:  "off",
			expectedLogOutput: true,
		},
		{
			name:              "explicit off",
			envValue:          "off",
			expectedLogLevel:  "off",
			expectedLogOutput: true,
		},
		{
			name:              "OFF uppercase",
			envValue:          "OFF",
			expectedLogLevel:  "off",
			expectedLogOutput: true,
		},
		{
			name:              "minimal logging",
			envValue:          "minimal",
			expectedLogLevel:  "minimal",
			expectedLogOutput: false,
		},
		{
			name:              "standard logging",
			envValue:          "standard",
			expectedLogLevel:  "standard",
			expectedLogOutput: false,
		},
		{
			name:              "verbose logging",
			envValue:          "verbose",
			expectedLogLevel:  "verbose",
			expectedLogOutput: false,
		},
		{
			name:              "custom logging",
			envValue:          "request,response",
			expectedLogLevel:  "request,response",
			expectedLogOutput: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envvar.AWSSDKLogLevel, tt.envValue)

			opts, err := buildValsOptions()
			require.NoError(t, err)

			assert.Equal(t, tt.expectedLogLevel, opts.AWSLogLevel)

			isDiscarded := opts.LogOutput == io.Discard
			assert.Equal(t, tt.expectedLogOutput, isDiscarded)
		})
	}
}

func TestBuildValsOptionsIntegration(t *testing.T) {
	t.Run("valid configuration produces working vals options", func(t *testing.T) {
		t.Setenv(envvar.AWSSDKLogLevel, "off")
		t.Setenv(envvar.ValsFailOnMissingKeyInMap, "true")

		opts, err := buildValsOptions()
		require.NoError(t, err)

		assert.Equal(t, valsCacheSize, opts.CacheSize)
		assert.Equal(t, "off", opts.AWSLogLevel)
		assert.True(t, opts.FailOnMissingKeyInMap)
		assert.Equal(t, io.Discard, opts.LogOutput)

		rt, err := vals.New(opts)
		require.NoError(t, err)
		assert.NotNil(t, rt)
	})
}
