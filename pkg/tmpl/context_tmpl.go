package tmpl

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
)

const recursionMaxNums = 1000

// CreateFuncMap creates a template.FuncMap for the Context struct.
// It combines the functions from sprig.TxtFuncMap() with the functions
// defined in the Context's createFuncMap() method.
// It also adds aliases for certain functions based on the aliases map.
// The resulting FuncMap is returned.
func (c *Context) CreateFuncMap() template.FuncMap {
	// function aliases
	aliased := template.FuncMap{}

	// map of function aliases
	aliases := map[string]string{
		"get": "sprigGet",
	}

	// get the default sprig functions
	funcMap := sprig.TxtFuncMap()

	// add aliases to the aliased FuncMap
	for orig, alias := range aliases {
		aliased[alias] = funcMap[orig]
	}

	// add functions from the Context's createFuncMap() method to the funcMap
	for name, f := range c.createFuncMap() {
		funcMap[name] = f
	}

	// add aliased functions to the funcMap
	for name, f := range aliased {
		funcMap[name] = f
	}

	return funcMap
}

type tplInfo struct {
	name    string
	content string
}

// helperTPLs returns the contents of all files with names starting with "_" and ending with ".tpl"
// in the root directory of the Context. It reads each file and appends its content to the contents slice.
// If any error occurs during the file reading or globbing process, it returns an error.
func (c *Context) helperTPLs() ([]tplInfo, error) {
	tplInfos := []tplInfo{}
	files, err := c.fs.Glob(filepath.Join(c.basePath, "_*.tpl"))
	if err != nil {
		return nil, fmt.Errorf("failed to glob helper templates: %v", err)
	}
	for _, file := range files {
		content, err := c.fs.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read helper template %s: %v", file, err)
		}
		tplInfos = append(tplInfos, tplInfo{name: file, content: string(content)})
	}
	return tplInfos, nil
}

// newTemplate creates a new template based on the context.
// It initializes the template with the specified options and parses the helper templates.
// It also adds the 'include' function to the template's function map.
// The 'include' function allows including and rendering nested templates.
// The function returns the created template or an error if any occurred.
func (c *Context) newTemplate() (*template.Template, error) {
	funcMap := c.CreateFuncMap()

	tmpl := template.New("stringTemplate")
	if c.preRender {
		tmpl = tmpl.Option("missingkey=zero")
	} else {
		tmpl = tmpl.Option("missingkey=error")
	}

	includedNames := make(map[string]int)

	// Add the 'include' function here so we can close over t.
	funcMap["include"] = func(name string, data interface{}) (string, error) {
		var buf strings.Builder
		if v, ok := includedNames[name]; ok {
			if v > recursionMaxNums {
				return "", errors.Wrapf(fmt.Errorf("unable to execute template"), "rendering template has a nested reference name: %s", name)
			}
			includedNames[name]++
		} else {
			includedNames[name] = 1
		}
		err := tmpl.ExecuteTemplate(&buf, name, data)
		includedNames[name]--
		return buf.String(), err
	}
	tmpl.Funcs(funcMap)

	tpls, err := c.helperTPLs()
	if err != nil {
		return nil, err
	}
	for _, tpl := range tpls {
		tmpl, err = tmpl.Parse(tpl.content)
		if err != nil {
			return nil, fmt.Errorf("failed to parse helper template %s: %v", tpl.name, err)
		}
	}

	return tmpl, nil
}

// RenderTemplateToBuffer renders the provided template string with the given data and returns the result as a *bytes.Buffer.
// The template string is parsed and executed using the Context's newTemplate method.
// If an error occurs during parsing or execution, it is returned along with the partially rendered template.
// The data parameter is optional and can be used to provide additional data for template rendering.
// If no data is provided, the template is rendered with an empty data context.
func (c *Context) RenderTemplateToBuffer(s string, data ...any) (*bytes.Buffer, error) {
	t, err := c.newTemplate()
	if err != nil {
		return nil, err
	}
	t, err = t.Parse(s)
	if err != nil {
		return nil, err
	}

	var tplString bytes.Buffer
	var d any
	if len(data) > 0 {
		d = data[0]
	}
	var execErr = t.Execute(&tplString, d)

	if execErr != nil {
		return &tplString, execErr
	}

	return &tplString, nil
}
