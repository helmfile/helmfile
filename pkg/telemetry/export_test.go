package telemetry

// Reset restores the pristine, disabled telemetry state. It exists for tests
// in this package and, through this export, for tests of other packages.
func Reset() { reset() }
