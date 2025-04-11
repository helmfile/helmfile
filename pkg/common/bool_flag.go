package common

// BoolFlag represents a boolean flag that tracks whether it was explicitly set
type BoolFlag interface {
    // Value returns the current boolean value
    Value() bool

    // WasExplicitlySet returns whether the flag was explicitly set
    WasExplicitlySet() bool

    // Set sets the value and marks the flag as explicitly set
    Set(value bool)
}

// boolFlag is the implementation of BoolFlag
type boolFlag struct {
    value          bool
    wasExplicitlySet bool
}

// NewBoolFlag creates a new BoolFlag with default values
func NewBoolFlag(defaultValue bool) BoolFlag {
    return &boolFlag{
        value: defaultValue,
        wasExplicitlySet: false,
    }
}

// Value returns the current boolean value
func (bf *boolFlag) Value() bool {
    return bf.value
}

// WasExplicitlySet returns whether the flag was explicitly set
func (bf *boolFlag) WasExplicitlySet() bool {
    return bf.wasExplicitlySet
}

// Set sets the value and marks the flag as explicitly set
func (bf *boolFlag) Set(value bool) {
    bf.value = value
    bf.wasExplicitlySet = true
}
