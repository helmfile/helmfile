package common

// MapFlag represents a string map flag that tracks whether it was explicitly set
type MapFlag interface {
	// Value returns a copy of the current map value
	Value() map[string]string

	// WasExplicitlySet returns whether the flag was explicitly set
	WasExplicitlySet() bool

	// Set sets the value and marks the flag as explicitly set
	Set(value map[string]string)

	// SetKey sets a specific key-value pair and marks the flag as explicitly set
	SetKey(key, value string)
}

// mapFlag is the implementation of MapFlag
type mapFlag struct {
	value            map[string]string
	wasExplicitlySet bool
}

// NewMapFlag creates a new MapFlag with default values
func NewMapFlag(defaultValue map[string]string) MapFlag {
	// Create a copy of the default value to avoid modifying the original
	valueCopy := make(map[string]string, len(defaultValue))
	for k, v := range defaultValue {
		valueCopy[k] = v
	}

	return &mapFlag{
		value:            valueCopy,
		wasExplicitlySet: false,
	}
}

// Value returns a copy of the current map value
func (mf *mapFlag) Value() map[string]string {
	// Create a copy to prevent external modification of internal state
	result := make(map[string]string, len(mf.value))
	for k, v := range mf.value {
		result[k] = v
	}
	return result
}

// WasExplicitlySet returns whether the flag was explicitly set
func (mf *mapFlag) WasExplicitlySet() bool {
	return mf.wasExplicitlySet
}

// Set sets the value and marks the flag as explicitly set
func (mf *mapFlag) Set(value map[string]string) {
	// Create a copy of the value to avoid modifying the original
	valueCopy := make(map[string]string, len(value))
	for k, v := range value {
		valueCopy[k] = v
	}

	mf.value = valueCopy
	mf.wasExplicitlySet = true
}

// SetKey sets a specific key-value pair and marks the flag as explicitly set
func (mf *mapFlag) SetKey(key, value string) {
	mf.value[key] = value
	mf.wasExplicitlySet = true
}
