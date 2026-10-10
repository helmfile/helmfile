package config

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestApplyImpl_TrackLogsInterval(t *testing.T) {
	tests := []struct {
		name    string
		setFlag string // explicit --track-logs-interval value; empty means not supplied
		noCmd   bool   // construct ApplyImpl without a Cmd, like non-cobra callers
		want    time.Duration
	}{
		{
			name: "zero when flag not supplied",
			want: 0,
		},
		{
			name:    "explicit value returned when flag supplied",
			setFlag: "3s",
			want:    3 * time.Second,
		},
		{
			name:    "explicit default value still counts as supplied",
			setFlag: "10s",
			want:    10 * time.Second,
		},
		{
			name:  "nil cmd falls back to option value",
			noCmd: true,
			want:  defaultTrackLogsInterval,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			options := NewApplyOptions()
			cmd := &cobra.Command{}
			cmd.Flags().DurationVar(&options.TrackLogsInterval, "track-logs-interval", options.TrackLogsInterval, "")
			if tt.setFlag != "" {
				require.NoError(t, cmd.Flags().Set("track-logs-interval", tt.setFlag))
			}

			impl := NewApplyImpl(nil, options)
			if !tt.noCmd {
				impl.Cmd = cmd
			}
			require.Equal(t, tt.want, impl.TrackLogsInterval())
		})
	}
}
