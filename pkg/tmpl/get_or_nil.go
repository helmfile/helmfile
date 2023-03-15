package tmpl

import (
	"fmt"
	"reflect"
	"strings"
)

type noValueError struct {
	msg string
}

func (e *noValueError) Error() string {
	return e.msg
}

func get(path string, varArgs ...interface{}) (interface{}, error) {
	var defSet bool
	var def interface{}
	var obj interface{}
	switch len(varArgs) {
	case 1:
		defSet = false
		def = nil
		obj = varArgs[0]
	case 2:
		defSet = true
		def = varArgs[0]
		obj = varArgs[1]
	default:
		return nil, fmt.Errorf("unexpected number of args passed to the template function get(path, [def, ]obj): expected 1 or 2, got %d, args was %v", len(varArgs), varArgs)
	}

	if path == "" {
		return obj, nil
	}
	keys := strings.Split(path, ".")
	var v interface{}
	var ok bool
	switch typedObj := obj.(type) {
	case *map[string]interface{}:
		obj = *typedObj
	}
	switch typedObj := obj.(type) {
	case map[string]interface{}:
		v, ok = typedObj[keys[0]]
		if !ok {
			if defSet {
				return def, nil
			}
			return nil, &noValueError{fmt.Sprintf("no value exist for key \"%s\" in %v", keys[0], typedObj)}
		}
	case map[string]string:
		v, ok = typedObj[keys[0]]
		if !ok {
			if defSet {
				return def, nil
			}
			return nil, &noValueError{fmt.Sprintf("no value exist for key \"%s\" in %v", keys[0], typedObj)}
		}
		return v, nil
	case map[interface{}]interface{}:
		v, ok = typedObj[keys[0]]
		if !ok {
			if defSet {
				return def, nil
			}
			return nil, &noValueError{fmt.Sprintf("no value exist for key \"%s\" in %v", keys[0], typedObj)}
		}
	default:
		maybeStruct := reflect.ValueOf(typedObj)
		if maybeStruct.Kind() != reflect.Struct {
			return nil, &noValueError{fmt.Sprintf("unexpected type(%v) of value for key \"%s\": it must be either map[string]interface{} or any struct", reflect.TypeOf(obj), keys[0])}
		} else if maybeStruct.NumField() < 1 {
			return nil, &noValueError{fmt.Sprintf("no accessible struct fields for key \"%s\"", keys[0])}
		}
		f := maybeStruct.FieldByName(keys[0])
		if !f.IsValid() {
			if defSet {
				return def, nil
			}
			return nil, &noValueError{fmt.Sprintf("no field named \"%s\" exist in %v", keys[0], typedObj)}
		}
		v = f.Interface()
	}

	if defSet {
		return get(strings.Join(keys[1:], "."), def, v)
	}
	return get(strings.Join(keys[1:], "."), v)
}

func getOrNil(path string, o interface{}) (interface{}, error) {
	v, err := get(path, o)
	if err != nil {
		switch err.(type) {
		case *noValueError:
			return nil, nil
		default:
			return nil, err
		}
	}
	return v, nil
}
