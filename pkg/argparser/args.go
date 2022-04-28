package argparser

import (
	"fmt"
	"strings"

	"github.com/helmfile/helmfile/pkg/state"
)

type keyVal struct {
	key       string
	val       string
	spaceFlag bool
}
type argMap struct {
	//m map[string]string
	m     map[string][]*keyVal
	flags []string
}

// SetArg sets a flag and value in the map
func (a *argMap) SetArg(flag, arg string, isSpace bool) {
	// if flag is empty, return
	if len(flag) == 0 {
		return
	}
	if _, exists := a.m[flag]; !exists {
		keyarg := &keyVal{key: flag, val: arg, spaceFlag: isSpace}
		a.m[flag] = append(a.m[flag], keyarg)
		a.flags = append(a.flags, flag)
	} else if flag == "--set" || flag == "-f" || flag == "--values" {
		keyarg := &keyVal{key: flag, val: arg, spaceFlag: isSpace}
		a.m[flag] = append(a.m[flag], keyarg)
	}
}

// newArgMap creates a new argMap
func newArgMap() *argMap {
	return &argMap{m: map[string][]*keyVal{}}
}

func GetArgs(args string, state *state.HelmState) []string {
	argsMap := newArgMap()

	if len(args) > 0 {
		argsVals := strings.Split(args, " ")
		for index, arg := range argsVals {
			if strings.HasPrefix(arg, "--") {
				argVal := strings.SplitN(arg, "=", 2)
				if len(argVal) > 1 {
					arg := argVal[0]
					value := argVal[1]
					argsMap.SetArg(arg, value, false)
				} else {
					//check if next value is arg to flag
					if index+1 < len(argsVals) {
						nextVal := argsVals[index+1]
						if strings.HasPrefix(nextVal, "--") {
							argsMap.SetArg(arg, "", false)
						} else {
							argsMap.SetArg(arg, nextVal, true)
						}
					} else {
						argsMap.SetArg(arg, "", false)
					}
				}
			}
		}
	}

	if len(state.HelmDefaults.Args) > 0 {
		for _, arg := range state.HelmDefaults.Args {
			var flag string
			var val string

			argsNum, _ := fmt.Sscanf(arg, "--%s %s", &flag, &val)
			if argsNum == 2 {
				argsMap.SetArg(flag, val, true)
			} else {
				argVal := strings.SplitN(arg, "=", 2)
				argFirst := argVal[0]
				if len(argVal) > 1 {
					val = argVal[1]
					argsMap.SetArg(argFirst, val, false)
				} else {
					argsMap.SetArg(argFirst, "", false)
				}
			}
		}
	}

	var argArr []string

	for _, flag := range argsMap.flags {
		val := argsMap.m[flag]

		for _, obj := range val {
			if obj.val != "" {
				if obj.spaceFlag {
					argArr = append(argArr, obj.key, obj.val)
				} else {
					argArr = append(argArr, fmt.Sprintf("%s=%s", obj.key, obj.val))
				}
			} else {
				argArr = append(argArr, obj.key)
			}
		}
	}

	state.HelmDefaults.Args = argArr

	return state.HelmDefaults.Args
}
