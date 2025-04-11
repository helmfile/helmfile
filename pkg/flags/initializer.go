package flags

import (
    "github.com/helmfile/helmfile/pkg/common"
)

// BoolFlagInitializer ensures a BoolFlag is initialized with a default value if nil
func EnsureBoolFlag(flag *common.BoolFlag, defaultValue bool) {
    if *flag == nil {
        *flag = common.NewBoolFlag(defaultValue)
    }
}

// StringFlagInitializer ensures a StringFlag is initialized with a default value if nil
func EnsureStringFlag(flag *common.StringFlag, defaultValue string) {
    if *flag == nil {
        *flag = common.NewStringFlag(defaultValue)
    }
}

// StringArrayFlagInitializer ensures a StringArrayFlag is initialized with default values if nil
func EnsureStringArrayFlag(flag *common.StringArrayFlag, defaultValues []string) {
    if *flag == nil {
        *flag = common.NewStringArrayFlag(defaultValues)
    }
}

// InitializeOptions initializes all nil flag fields in an options struct
func InitializeOptions(options interface{}) {
    // This could be expanded to use reflection to automatically find and initialize
    // all flag fields in any options struct
}