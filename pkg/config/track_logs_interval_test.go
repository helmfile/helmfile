package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTrackLogsIntervalDefault(t *testing.T) {
	require.Equal(t, 10*time.Second, NewApplyOptions().TrackLogsInterval)
	require.Equal(t, 10*time.Second, NewSyncOptions().TrackLogsInterval)
}

func TestTrackLogsIntervalValidation(t *testing.T) {
	commands := []struct {
		name     string
		validate func(time.Duration) error
	}{
		{
			name: "apply",
			validate: func(interval time.Duration) error {
				options := NewApplyOptions()
				options.TrackLogsInterval = interval
				return NewApplyImpl(NewGlobalImpl(&GlobalOptions{}), options).ValidateConfig()
			},
		},
		{
			name: "sync",
			validate: func(interval time.Duration) error {
				options := NewSyncOptions()
				options.TrackLogsInterval = interval
				return NewSyncImpl(NewGlobalImpl(&GlobalOptions{}), options).ValidateConfig()
			},
		},
	}

	for _, command := range commands {
		t.Run(command.name, func(t *testing.T) {
			for _, tt := range []struct {
				name     string
				interval time.Duration
				wantErr  bool
			}{
				{name: "zero", interval: 0, wantErr: true},
				{name: "below minimum", interval: 500 * time.Millisecond, wantErr: true},
				{name: "minimum", interval: time.Second},
			} {
				t.Run(tt.name, func(t *testing.T) {
					err := command.validate(tt.interval)
					if tt.wantErr {
						require.ErrorContains(t, err, "--track-logs-interval must be at least 1s")
					} else {
						require.NoError(t, err)
					}
				})
			}
		})
	}
}
