package common

import (
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestNewBoolFlag(t *testing.T) {
    tests := []struct {
        name         string
        defaultValue bool
        expected     bool
    }{
        {
            name:         "default true",
            defaultValue: true,
            expected:     true,
        },
        {
            name:         "default false",
            defaultValue: false,
            expected:     false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            flag := NewBoolFlag(tt.defaultValue)

            // Check initial state
            assert.Equal(t, tt.expected, flag.Value(), "Value should match default")
            assert.False(t, flag.WasExplicitlySet(), "New flag should not be marked as explicitly set")
        })
    }
}

func TestBoolFlag_Set(t *testing.T) {
    tests := []struct {
        name         string
        defaultValue bool
        setValue     bool
        expected     bool
    }{
        {
            name:         "default false, set true",
            defaultValue: false,
            setValue:     true,
            expected:     true,
        },
        {
            name:         "default true, set false",
            defaultValue: true,
            setValue:     false,
            expected:     false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            flag := NewBoolFlag(tt.defaultValue)

            // Set the value
            flag.Set(tt.setValue)

            // Check state after setting
            assert.Equal(t, tt.expected, flag.Value(), "Value should match set value")
            assert.True(t, flag.WasExplicitlySet(), "Flag should be marked as explicitly set")
        })
    }
}

func TestBoolFlag_MultipleSet(t *testing.T) {
    flag := NewBoolFlag(false)

    // Initial state
    assert.False(t, flag.Value())
    assert.False(t, flag.WasExplicitlySet())

    // First set
    flag.Set(true)
    assert.True(t, flag.Value())
    assert.True(t, flag.WasExplicitlySet())

    // Second set
    flag.Set(false)
    assert.False(t, flag.Value())
    assert.True(t, flag.WasExplicitlySet(), "Flag should remain explicitly set")
}

func TestBoolFlag_Implementation(t *testing.T) {
    // Test that boolFlag properly implements BoolFlag interface
    var _ BoolFlag = &boolFlag{}
}
