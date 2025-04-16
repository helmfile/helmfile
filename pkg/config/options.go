package config

// Options is the base interface for all command options
type Options interface {
	// Initialize sets default values for options
	Initialize()
}

// FlagHandler handles flag values from command line
type FlagHandler interface {
	// HandleFlag processes a flag value
	HandleFlag(name string, value interface{}, changed bool)
}
