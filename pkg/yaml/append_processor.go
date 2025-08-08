package yaml

import (
	"strings"

	"dario.cat/mergo"
)

type AppendProcessor struct{}

func NewAppendProcessor() *AppendProcessor {
	return &AppendProcessor{}
}

func (ap *AppendProcessor) MergeWithAppend(dest, src map[string]any) error {
	convertToStringMapInPlace(dest)
	convertToStringMapInPlace(src)

	appendMap := make(map[string]any)
	regularMap := make(map[string]any)

	for key, value := range src {
		if IsAppendKey(key) {
			baseKey := GetBaseKey(key)
			appendMap[baseKey] = value
		} else {
			regularMap[key] = value
		}
	}

	if len(appendMap) > 0 {
		for baseKey, appendValue := range appendMap {
			destValue, exists := dest[baseKey]
			if exists {
				if _, ok := destValue.([]any); !ok {
					dest[baseKey] = appendValue
					delete(appendMap, baseKey)
				}
			}
		}

		if len(appendMap) > 0 {
			tempDest := make(map[string]any)
			for k, v := range dest {
				tempDest[k] = v
			}

			if err := mergo.Merge(&tempDest, appendMap, mergo.WithAppendSlice, mergo.WithSliceDeepCopy); err != nil {
				return err
			}

			for k, v := range tempDest {
				dest[k] = v
			}
		}
	}

	for key, value := range regularMap {
		if srcMap, ok := value.(map[string]any); ok {
			if destMap, ok := dest[key].(map[string]any); ok {
				if err := ap.MergeWithAppend(destMap, srcMap); err != nil {
					return err
				}
				dest[key] = destMap
			} else {
				newDestMap := make(map[string]any)
				if err := ap.MergeWithAppend(newDestMap, srcMap); err != nil {
					return err
				}
				dest[key] = newDestMap
			}
		} else {
			tempDest := make(map[string]any)
			tempDest[key] = dest[key]
			tempSrc := make(map[string]any)
			tempSrc[key] = value

			if err := mergo.Merge(&tempDest, tempSrc, mergo.WithOverride); err != nil {
				return err
			}

			dest[key] = tempDest[key]
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

func IsAppendKey(key string) bool {
	return strings.HasSuffix(key, "+")
}

func GetBaseKey(key string) string {
	return strings.TrimSuffix(key, "+")
}
