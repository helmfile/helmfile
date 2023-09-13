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
	m     map[string][]*keyVal
	flags []string
}
type GetArgsOptions struct {
	WithDiffArgs bool
}

// isNewFlag checks if the given arg is a new flag
func isNewFlag(flag string) bool {
	return strings.HasPrefix(flag, "--") || strings.HasPrefix(flag, "-")
}

// removeEmptyArgs removes empty args from the given args
func removeEmptyArgs(args []string) []string {
	var newArgs []string
	for _, arg := range args {
		if len(arg) > 0 {
			newArgs = append(newArgs, arg)
		}
	}
	return newArgs
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
	} else {
		keyarg := &keyVal{key: flag, val: arg, spaceFlag: isSpace}
		a.m[flag] = append(a.m[flag], keyarg)
	}
}

// newArgMap creates a new argMap
func newArgMap() *argMap {
	return &argMap{m: map[string][]*keyVal{}}
}

func analyzeArgs(am *argMap, args string) {
	argsVals := removeEmptyArgs(strings.Split(args, " "))
	for index, arg := range argsVals {
		if len(arg) == 0 {
			continue
		}
		if isNewFlag(arg) {
			argVal := strings.SplitN(arg, "=", 2)
			if len(argVal) > 1 {
				arg := argVal[0]
				value := argVal[1]
				am.SetArg(arg, value, false)
			} else {
				// check if next value is arg to flag
				if index+1 < len(argsVals) {
					nextVal := argsVals[index+1]
					if isNewFlag(nextVal) {
						am.SetArg(arg, "", false)
					} else {
						am.SetArg(arg, nextVal, true)
					}
				} else {
					am.SetArg(arg, "", false)
				}
			}
		}
	}
}

func GetArgs(args string, state *state.HelmState, opts *GetArgsOptions) []string {
	argsMap := newArgMap()

	if len(args) > 0 {
		analyzeArgs(argsMap, args)
	}

	if len(state.HelmDefaults.Args) > 0 {
		analyzeArgs(argsMap, strings.Join(state.HelmDefaults.Args, " "))
	}

	if len(state.HelmDefaults.DiffArgs) > 0 && opts != nil && opts.WithDiffArgs {
		analyzeArgs(argsMap, strings.Join(state.HelmDefaults.DiffArgs, " "))
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
