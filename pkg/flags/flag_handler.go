package flags

// FlagHandler is a generic interface for handling flag values
type FlagHandler interface {
	// HandleFlag receives a flag name, value, and whether it was changed
	// Returns true if the flag was handled, false otherwise
	HandleFlag(name string, value interface{}, changed bool) bool
}
