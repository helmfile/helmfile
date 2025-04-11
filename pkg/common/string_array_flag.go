


package common

// For array/slice flags
type StringArrayFlag interface {
    Values() []string
    WasExplicitlySet() bool
    Add(value string)
    Set(values []string)
}

type stringArrayFlag struct {
    values           []string
    wasExplicitlySet bool
}

// NewStringArrayFlag creates a new StringArrayFlag with the given default values
// Create a defensive copy of the default values to prevent external modifications
// and to ensure that the original values remain unchanged.
func NewStringArrayFlag(defaultValues []string) StringArrayFlag {
    valuesCopy := make([]string, len(defaultValues))
    copy(valuesCopy, defaultValues)

    return &stringArrayFlag{
        values: valuesCopy,
        wasExplicitlySet: false,
    }
}
// Values returns the values of the flag
// It returns a copy of the values to prevent external modifications
// and to ensure that the original values remain unchanged.
// This is important for flags that may be modified by the user
// or other parts of the program.
func (f *stringArrayFlag) Values() []string {
    // Return a copy to prevent external modifications
    valuesCopy := make([]string, len(f.values))
    copy(valuesCopy, f.values)
    return valuesCopy
}

// WasExplicitlySet returns whether the flag was explicitly set
func (f *stringArrayFlag) WasExplicitlySet() bool {
    return f.wasExplicitlySet
}

// Set sets the values and marks the flag as explicitly set
func (f *stringArrayFlag) Set(values []string) {
    f.values = values
    f.wasExplicitlySet = true
}

// Add sets the value and marks the flag as explicitly set
func (f *stringArrayFlag) Add(value string) {
	f.values = append(f.values, value)
    f.wasExplicitlySet = true
}
