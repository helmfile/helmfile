package yaml

import (
	"strings"
)

type AppendProcessor struct{}

func NewAppendProcessor() *AppendProcessor {
	return &AppendProcessor{}
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
