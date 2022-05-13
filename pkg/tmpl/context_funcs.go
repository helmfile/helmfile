package tmpl

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"github.com/ghodss/yaml"
	"github.com/roboll/helmfile/pkg/helmexec"
	"golang.org/x/sync/errgroup"
)

type Values = map[string]interface{}

func (c *Context) createFuncMap() template.FuncMap {
	funcMap := template.FuncMap{
		"exec":             c.Exec,
		"isFile":           c.IsFile,
		"readFile":         c.ReadFile,
		"readDir":          c.ReadDir,
		"toYaml":           ToYaml,
		"fromYaml":         FromYaml,
		"setValueAtPath":   SetValueAtPath,
		"requiredEnv":      RequiredEnv,
		"get":              get,
		"getOrNil":         getOrNil,
		"tpl":              c.Tpl,
		"required":         Required,
		"setEnv":           SetEnv,
		"fetchSecretValue": fetchSecretValue,
		"expandSecretRefs": fetchSecretValues,
	}
	if c.preRender {
		// disable potential side-effect template calls
		funcMap["exec"] = func(string, []interface{}, ...string) (string, error) {
			return "", nil
		}
		funcMap["readFile"] = func(string) (string, error) {
			return "", nil
		}
	}

	return funcMap
}

func (c *Context) Exec(command string, args []interface{}, inputs ...string) (string, error) {
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

	cmd := exec.Command(command, strArgs...)
	cmd.Dir = c.basePath

	g := errgroup.Group{}

	if len(input) > 0 {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return "", err
		}

		g.Go(func() error {
			defer stdin.Close()

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

	bytes, err := c.readFile(path)
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

	entries, err := os.ReadDir(contextPath)
	if err != nil {
		return nil, fmt.Errorf("ReadDir %q: %w", contextPath, err)
	}

	var filenames []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filenames = append(filenames, filepath.Join(path, entry.Name()))
	}

	return filenames, nil
}

func (c *Context) Tpl(text string, data interface{}) (string, error) {
	buf, err := c.RenderTemplateToBuffer(text, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func ToYaml(v interface{}) (string, error) {
	data, err := yaml.Marshal(v)
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

// SetEnv sets an environment variable.
func SetEnv(name string, value interface{}) error {
	if err := os.Setenv(name, fmt.Sprintf("%v", value)); err != nil {
		return fmt.Errorf("failed to set env var `%s`: %w", name, err)
	}
	return nil
}
