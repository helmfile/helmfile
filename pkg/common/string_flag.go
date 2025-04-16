package common

type StringFlag interface {
	Value() string
	WasExplicitlySet() bool
	Set(value string)
}

type stringFlag struct {
	value            string
	wasExplicitlySet bool
}

func NewStringFlag(defaultValue string) StringFlag {
	return &stringFlag{
		value:            defaultValue,
		wasExplicitlySet: false,
	}
}

// Value returns the current boolean value
func (f *stringFlag) Value() string {
	return f.value
}

// WasExplicitlySet returns whether the flag was explicitly set
func (f *stringFlag) WasExplicitlySet() bool {
	return f.wasExplicitlySet
}

// Set sets the value and marks the flag as explicitly set
func (f *stringFlag) Set(value string) {
	f.value = value
	f.wasExplicitlySet = true
}
