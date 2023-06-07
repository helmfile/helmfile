package helmexec

import (
	"errors"
	"testing"
)

func TestNewExitError(t *testing.T) {
	for _, tt := range []struct {
		name                       string
		stripArgsValuesOnExitError bool
		want                       string
	}{
		{
			name:                       "newExitError with stripArgsValuesOnExitError false",
			stripArgsValuesOnExitError: false,
			want: `command "helm" exited with non-zero status:

PATH:
  helm

ARGS:
  0: --set (5 bytes)
  1: a=b (3 bytes)
  2: --set-string (12 bytes)
  3: a=b (3 bytes)

ERROR:
  test

EXIT STATUS
  1`,
		},
		{
			name:                       "newExitError with stripArgsValuesOnExitError true",
			stripArgsValuesOnExitError: true,
			want: `command "helm" exited with non-zero status:

PATH:
  helm

ARGS:
  0: --set (5 bytes)
  1: *** STRIP *** (13 bytes)
  2: --set-string (12 bytes)
  3: *** STRIP *** (13 bytes)

ERROR:
  test

EXIT STATUS
  1`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			exitError := newExitError("helm", []string{"--set", "a=b", "--set-string", "a=b"}, 1, errors.New("test"), "", "", tt.stripArgsValuesOnExitError)
			if want, have := tt.want, exitError.Error(); want != have {
				t.Errorf("want %q, have %q", want, have)
			}
		})
	}
}
