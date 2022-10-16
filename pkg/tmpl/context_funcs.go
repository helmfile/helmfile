package tmpl

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/maputil"
)

type Values = map[string]interface{}

var DisableInsecureFeaturesErr = DisableInsecureFeaturesError{envvar.DisableInsecureFeatures + " is active, insecure function calls are disabled"}

type DisableInsecureFeaturesError struct {
	err string
}

func (e DisableInsecureFeaturesError) Error() string {
	return e.err
}

var (
	disableInsecureFeatures       bool
	skipInsecureTemplateFunctions bool
)

func init() {
	disableInsecureFeatures, _ = strconv.ParseBool(os.Getenv(envvar.DisableInsecureFeatures))
	skipInsecureTemplateFunctions, _ = strconv.ParseBool(os.Getenv(envvar.SkipInsecureTemplateFunctions))
}

func (c *Context) createFuncMap() template.FuncMap {
	funcMap := template.FuncMap{
		"envExec":          c.EnvExec,
		"exec":             c.Exec,
		"isFile":           c.IsFile,
		"readFile":         c.ReadFile,
		"readDir":          c.ReadDir,
		"readDirEntries":   c.ReadDirEntries,
		"toYaml":           ToYaml,
		"fromYaml":         FromYaml,
		"setValueAtPath":   SetValueAtPath,
		"requiredEnv":      RequiredEnv,
		"get":              get,
		"getOrNil":         getOrNil,
		"tpl":              c.Tpl,
		"required":         Required,
		"fetchSecretValue": fetchSecretValue,
		"expandSecretRefs": fetchSecretValues,
	}
	if c.preRender || skipInsecureTemplateFunctions {
		// disable potential side-effect template calls
		funcMap["exec"] = func(string, []interface{}, ...string) (string, error) {
			return "", nil
		}
		funcMap["envExec"] = func(map[string]interface{}, string, []interface{}, ...string) (string, error) {
			return "", nil
		}
		funcMap["readFile"] = func(string) (string, error) {
			return "", nil
		}
		funcMap["readDir"] = func(string) ([]string, error) {
			return []string{}, nil
		}
		funcMap["readDirEntries"] = func(string) ([]fs.DirEntry, error) {
			return []fs.DirEntry{}, nil
		}
	}
	if disableInsecureFeatures {
		// disable insecure functions
		funcMap["exec"] = func(string, []interface{}, ...string) (string, error) {
			return "", DisableInsecureFeaturesErr
		}
		funcMap["readFile"] = func(string) (string, error) {
			return "", DisableInsecureFeaturesErr
		}
		funcMap["readDir"] = func(string) ([]string, error) {
			return nil, DisableInsecureFeaturesErr
		}
		funcMap["readDirEntries"] = func(string) ([]string, error) {
			return nil, DisableInsecureFeaturesErr
		}
	}

	return funcMap
}

// TODO: in the next major version, remove this function.
func (c *Context) EnvExec(envs map[string]interface{}, command string, args []interface{}, inputs ...string) (string, error) {
	var input string
	if len(inputs) > 0 {
		input = inputs[0]
	}

	strArgs := make([]string, len(args))
	for i, a := range args {
		switch a.(type) {
		case string:
			strArgs[i] = fmt.Sprintf("%v", a)
		default:
			return "", fmt.Errorf("unexpected type of arg \"%s\" in args %v at index %d", reflect.TypeOf(a), args, i)
		}
	}

	envsLen := len(envs)
	strEnvs := make(map[string]string, envsLen)

	for k, v := range envs {
		switch v.(type) {
		case string:
			strEnvs[k] = fmt.Sprintf("%v", v)
		default:
			return "", fmt.Errorf("unexpected type of env \"%s\" in envs %v at index %s", reflect.TypeOf(v), envs, k)
		}
	}

	cmd := exec.Command(command, strArgs...)
	cmd.Dir = c.basePath
	if envs != nil {
		cmd.Env = os.Environ()
		for k, v := range envs {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	g := errgroup.Group{}

	if len(input) > 0 {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return "", err
		}

		g.Go(func() error {
			defer func() {
				_ = stdin.Close()
			}()

			size := len(input)

			i := 0

			for {
				n, err := io.WriteString(stdin, input[i:])
				if err != nil {
					return fmt.Errorf("failed while writing %d bytes to stdin of \"%s\": %v", len(input), command, err)
				}

				i += n

				if i == size {
					return nil
				}
			}
		})
	}

	var bytes []byte

	g.Go(func() error {
		// We use CombinedOutput to produce helpful error messages
		// See https://github.com/roboll/helmfile/issues/1158
		bs, err := helmexec.Output(cmd)
		if err != nil {
			return err
		}

		bytes = bs

		return nil
	})

	if err := g.Wait(); err != nil {
		return "", err
	}

	return string(bytes), nil
}

func (c *Context) Exec(command string, args []interface{}, inputs ...string) (string, error) {
	return c.EnvExec(nil, command, args, inputs...)
}

func (c *Context) IsFile(filename string) (bool, error) {
	var path string
	if filepath.IsAbs(filename) {
		path = filename
	} else {
		path = filepath.Join(c.basePath, filename)
	}

	stat, err := os.Stat(path)
	if err == nil {
		return !stat.IsDir(), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (c *Context) ReadFile(filename string) (string, error) {
	var path string
	if filepath.IsAbs(filename) {
		path = filename
	} else {
		path = filepath.Join(c.basePath, filename)
	}

	if c.fs.ReadFile == nil {
		return "", fmt.Errorf("readFile is not implemented")
	}

	bytes, err := c.fs.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (c *Context) ReadDir(path string) ([]string, error) {
	var contextPath string
	if filepath.IsAbs(path) {
		contextPath = path
	} else {
		contextPath = filepath.Join(c.basePath, path)
	}

	entries, err := c.fs.ReadDir(contextPath)
	if err != nil {
		return nil, fmt.Errorf("ReadDir %q: %w", contextPath, err)
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		paths = append(paths, filepath.Join(path, entry.Name()))
	}

	return paths, nil
}

func (c *Context) ReadDirEntries(path string) ([]fs.DirEntry, error) {
	var contextPath string
	if filepath.IsAbs(path) {
		contextPath = path
	} else {
		contextPath = filepath.Join(c.basePath, path)
	}
	entries, err := c.fs.ReadDir(contextPath)
	if err != nil {
		return nil, fmt.Errorf("ReadDirEntries %q: %w", contextPath, err)
	}
	return entries, nil
}

func (c *Context) Tpl(text string, data interface{}) (string, error) {
	buf, err := c.RenderTemplateToBuffer(text, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func ToYaml(v interface{}) (string, error) {
	data, err := maputil.YamlMarshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func FromYaml(str string) (Values, error) {
	m := Values{}

	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		return nil, fmt.Errorf("%s, offending yaml: %s", err, str)
	}
	return m, nil
}

func SetValueAtPath(path string, value interface{}, values Values) (Values, error) {
	var current interface{}
	current = values
	components := strings.Split(path, ".")
	pathToMap := components[:len(components)-1]
	key := components[len(components)-1]
	for _, k := range pathToMap {
		var elem interface{}

		switch typedCurrent := current.(type) {
		case map[string]interface{}:
			v, exists := typedCurrent[k]
			if !exists {
				return nil, fmt.Errorf("failed to set value at path \"%s\": value for key \"%s\" does not exist", path, k)
			}
			elem = v
		case map[interface{}]interface{}:
			v, exists := typedCurrent[k]
			if !exists {
				return nil, fmt.Errorf("failed to set value at path \"%s\": value for key \"%s\" does not exist", path, k)
			}
			elem = v
		default:
			return nil, fmt.Errorf("failed to set value at path \"%s\": value for key \"%s\" was not a map", path, k)
		}

		switch typedElem := elem.(type) {
		case map[string]interface{}, map[interface{}]interface{}:
			current = typedElem
		default:
			return nil, fmt.Errorf("failed to set value at path \"%s\": value for key \"%s\" was not a map", path, k)
		}
	}

	switch typedCurrent := current.(type) {
	case map[string]interface{}:
		typedCurrent[key] = value
	case map[interface{}]interface{}:
		typedCurrent[key] = value
	default:
		return nil, fmt.Errorf("failed to set value at path \"%s\": value for key \"%s\" was not a map", path, key)
	}
	return values, nil
}

func RequiredEnv(name string) (string, error) {
	if val, exists := os.LookupEnv(name); exists && len(val) > 0 {
		return val, nil
	}

	return "", fmt.Errorf("required env var `%s` is not set", name)
}

func Required(warn string, val interface{}) (interface{}, error) {
	if val == nil {
		return nil, fmt.Errorf(warn)
	} else if _, ok := val.(string); ok {
		if val == "" {
			return nil, fmt.Errorf(warn)
		}
	}

	return val, nil
}
