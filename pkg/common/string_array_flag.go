package common

// StringArrayFlag represents a string array flag that tracks whether it was explicitly set
type StringArrayFlag interface {
	// Value returns a copy of the current string array value
	Value() []string

	// WasExplicitlySet returns whether the flag was explicitly set
	WasExplicitlySet() bool

	// Set sets the value and marks the flag as explicitly set
	Set(value []string)

	// Append adds a value to the array and marks the flag as explicitly set
	Append(value string)
}

// stringArrayFlag is the implementation of ArrayFlag
type stringArrayFlag struct {
	value            []string
	wasExplicitlySet bool
}

// NewStringArrayFlag creates a new ArrayFlag with default values
func NewStringArrayFlag(defaultValue []string) StringArrayFlag {
	// Create a copy of the default value to avoid modifying the original
	valueCopy := make([]string, len(defaultValue))
	copy(valueCopy, defaultValue)

	return &stringArrayFlag{
		value:            valueCopy,
		wasExplicitlySet: false,
	}
}

// Value returns a copy of the current string array value
func (af *stringArrayFlag) Value() []string {
	// Create a copy to prevent external modification of internal state
	result := make([]string, len(af.value))
	copy(result, af.value)
	return result
}

// WasExplicitlySet returns whether the flag was explicitly set
func (af *stringArrayFlag) WasExplicitlySet() bool {
	return af.wasExplicitlySet
}

// Set sets the value and marks the flag as explicitly set
func (af *stringArrayFlag) Set(value []string) {
	// Create a copy of the value to avoid modifying the original
	valueCopy := make([]string, len(value))
	copy(valueCopy, value)

	af.value = valueCopy
	af.wasExplicitlySet = true
}

// Append adds a value to the array and marks the flag as explicitly set
func (af *stringArrayFlag) Append(value string) {
	af.value = append(af.value, value)
	af.wasExplicitlySet = true
}
