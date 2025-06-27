package yaml

import (
	"fmt"
	"strings"
)

type AppendProcessor struct{}

func NewAppendProcessor() *AppendProcessor {
	return &AppendProcessor{}
}

func (ap *AppendProcessor) ProcessMap(data map[string]any) (map[string]any, error) {
	result := make(map[string]any)

	// First pass: collect all append keys and their base keys
	appendKeys := make(map[string][]any)
	baseKeys := make(map[string]any)

	for key, value := range data {
		if IsAppendKey(key) {
			baseKey := GetBaseKey(key)
			appendKeys[baseKey] = append(appendKeys[baseKey], value)
		} else {
			baseKeys[key] = value
		}
	}

	// Second pass: process all values recursively
	for key, value := range baseKeys {
		processedValue, err := ap.processValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to process value for key %s: %w", key, err)
		}
		result[key] = processedValue
	}

	// Third pass: merge append keys with their base keys
	for baseKey, appendValues := range appendKeys {
		for _, appendValue := range appendValues {
			processedValue, err := ap.processValue(appendValue)
			if err != nil {
				return nil, fmt.Errorf("failed to process append value for key %s: %w", baseKey, err)
			}
			if existingValue, exists := result[baseKey]; exists {
				if isSlice(processedValue) && isSlice(existingValue) {
					// Always append to the base key's slice
					result[baseKey] = append(existingValue.([]any), processedValue.([]any)...)
				} else {
					// If not both slices, overwrite (fallback)
					result[baseKey] = processedValue
				}
			} else {
				result[baseKey] = processedValue
			}
		}
	}

	return result, nil
}

func (ap *AppendProcessor) processValue(value any) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		return ap.ProcessMap(v)
	case map[any]any:
		converted := make(map[string]any)
		for k, val := range v {
			if strKey, ok := k.(string); ok {
				converted[strKey] = val
			} else {
				return nil, fmt.Errorf("non-string key in map: %v", k)
			}
		}
		return ap.ProcessMap(converted)
	case []any:
		result := make([]any, len(v))
		for i, elem := range v {
			processed, err := ap.processValue(elem)
			if err != nil {
				return nil, fmt.Errorf("failed to process slice element %d: %w", i, err)
			}
			result[i] = processed
		}
		return result, nil
	default:
		return value, nil
	}
}

func (ap *AppendProcessor) MergeWithAppend(dest, src map[string]any) error {
	convertToStringMapInPlace(dest)
	convertToStringMapInPlace(src)

	for key, srcValue := range src {
		if IsAppendKey(key) {
			baseKey := GetBaseKey(key)
			destValue, exists := dest[baseKey]
			if exists {
				if isSlice(srcValue) && isSlice(destValue) {
					destSlice := destValue.([]any)
					srcSlice := srcValue.([]any)
					dest[baseKey] = append(destSlice, srcSlice...)
				} else {
					dest[baseKey] = srcValue
				}
			} else {
				dest[baseKey] = srcValue
			}
			delete(src, key)
		}
	}

	for key, srcValue := range src {
		if isMap(srcValue) {
			srcMap := srcValue.(map[string]any)
			if destMap, ok := dest[key].(map[string]any); ok {
				if err := ap.MergeWithAppend(destMap, srcMap); err != nil {
					return err
				}
				dest[key] = destMap
			} else {
				dest[key] = srcMap
			}
		} else {
			dest[key] = srcValue
		}
	}
	return nil
}

func convertToStringMapInPlace(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, v2 := range t {
			t[k] = convertToStringMapInPlace(v2)
		}
		return t
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, v2 := range t {
			if ks, ok := k.(string); ok {
				m[ks] = convertToStringMapInPlace(v2)
			}
		}
		return m
	case []any:
		for i, v2 := range t {
			t[i] = convertToStringMapInPlace(v2)
		}
		return t
	default:
		return v
	}
}

func isSlice(value any) bool {
	_, ok := value.([]any)
	return ok
}

func isMap(value any) bool {
	_, ok := value.(map[string]any)
	return ok
}

func IsAppendKey(key string) bool {
	return strings.HasSuffix(key, "+")
}

func GetBaseKey(key string) string {
	return strings.TrimSuffix(key, "+")
}
