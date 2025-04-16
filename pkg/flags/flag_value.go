package flags

// GetFlagValue is a generic function to get flag values with type safety
func GetFlagValue[T any](registry FlagRegistry, name string) (T, bool) {
	var zero T
	values := registry.GetValues()
	if value, exists := values[name]; exists {
		if typedValue, ok := value.(*T); ok {
			return *typedValue, true
		}
	}
	return zero, false
}

// GetBoolFlagValue is a convenience function to get a boolean flag value
func GetBoolFlagValue(registry FlagRegistry, name string) (bool, bool) {
	return GetFlagValue[bool](registry, name)
}

// GetStringFlagValue is a convenience function to get a string flag value
func GetStringFlagValue(registry FlagRegistry, name string) (string, bool) {
	return GetFlagValue[string](registry, name)
}

// GetStringSliceFlagValue is a convenience function to get a string slice flag value
func GetStringSliceFlagValue(registry FlagRegistry, name string) ([]string, bool) {
	return GetFlagValue[[]string](registry, name)
}

// GetIntFlagValue is a convenience function to get an integer flag value
func GetIntFlagValue(registry FlagRegistry, name string) (int, bool) {
	return GetFlagValue[int](registry, name)
}

// GetInt64FlagValue is a convenience function to get an int64 flag value
func GetInt64FlagValue(registry FlagRegistry, name string) (int64, bool) {
	return GetFlagValue[int64](registry, name)
}

// GetFloat64FlagValue is a convenience function to get a float64 flag value
func GetFloat64FlagValue(registry FlagRegistry, name string) (float64, bool) {
	return GetFlagValue[float64](registry, name)
}
