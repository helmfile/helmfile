package state

import (
	"fmt"
)

const ReleaseErrorCodeFailure = 1

type ReleaseError struct {
	*ReleaseSpec
	err  error
	Code int
}

func (e *ReleaseError) Error() string {
	// e.err is nil for synthetic errors that signal a helm-diff exit
	// status rather than a failure (see DiffReleases); formatting those
	// must not panic.
	if e.ReleaseSpec == nil {
		return fmt.Sprintf("release error: exit code %d", e.Code)
	}
	if e.err == nil {
		return fmt.Sprintf("release %s: exit code %d", e.Name, e.Code)
	}
	return e.err.Error()
}

func NewReleaseError(release *ReleaseSpec, err error, code int) *ReleaseError {
	return &ReleaseError{
		ReleaseSpec: release,
		err:         err,
		Code:        code,
	}
}

func newReleaseFailedError(release *ReleaseSpec, err error) *ReleaseError {
	wrappedErr := fmt.Errorf("failed processing release %s: %v", release.Name, err.Error())

	return NewReleaseError(release, wrappedErr, ReleaseErrorCodeFailure)
}
