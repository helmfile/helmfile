package app

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/state"
	"github.com/helmfile/helmfile/pkg/tmpl"
)

func prependLineNumbers(text string) string {
	buf := bytes.NewBufferString("")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		buf.WriteString(fmt.Sprintf("%2d: %s\n", i, line))
	}
	return buf.String()
}

func (r *desiredStateLoader) renderPrestate(firstPassEnv *environment.Environment, baseDir, filename string, content []byte) (*environment.Environment, *state.HelmState) {
	tmplData := state.NewEnvironmentTemplateData(*firstPassEnv, r.namespace, map[string]interface{}{})
	firstPassRenderer := tmpl.NewFirstPassRenderer(baseDir, tmplData)

	// parse as much as we can, tolerate errors, this is a preparse
	yamlBuf, err := firstPassRenderer.RenderTemplateContentToBuffer(content)
	if err != nil && r.logger != nil {
		r.logger.Debugf("first-pass rendering input of \"%s\":\n%s", filename, prependLineNumbers(string(content)))
		r.logger.Debugf("template syntax error: %v", err)
		if yamlBuf == nil { // we have a template syntax error, let the second parse report
			return firstPassEnv, nil
		}
	}
	yamlData := yamlBuf.String()
	if r.logger != nil {
		r.logger.Debugf("first-pass rendering output of \"%s\":\n%s", filename, prependLineNumbers(yamlData))
	}

	// Work-around for https://github.com/golang/go/issues/24963
	sanitized := strings.ReplaceAll(yamlData, "<no value>", "")

	if len(yamlData) != len(sanitized) {
		msg := "replaced <no value>s to workaround https://github.com/golang/go/issues/24963 to address https://github.com/roboll/helmfile/issues/553:\n%s"
		r.logger.Debugf(msg, cmp.Diff(yamlData, sanitized))
	}

	c := r.underlying()
	c.Strict = false
	// create preliminary state, as we may have an environment. Tolerate errors.
	prestate, err := c.ParseAndLoad([]byte(sanitized), baseDir, filename, r.env, false, firstPassEnv)
	if err != nil && r.logger != nil {
		if _, ok := err.(*state.StateLoadError); ok {
			r.logger.Debugf("could not deduce `environment:` block, configuring only .Environment.Name. error: %v", err)
		}
		r.logger.Debugf("error in first-pass rendering: result of \"%s\":\n%s", filename, prependLineNumbers(yamlBuf.String()))
	}

	if prestate != nil {
		firstPassEnv = &prestate.Env
	}

	return firstPassEnv, prestate
}

type RenderOpts struct {
}

func (r *desiredStateLoader) renderTemplatesToYamlWithEnv(baseDir, filename string, content []byte, inherited, overrode *environment.Environment) (*bytes.Buffer, error) {
	return r.twoPassRenderTemplateToYaml(inherited, overrode, baseDir, filename, content)
}

func (r *desiredStateLoader) twoPassRenderTemplateToYaml(inherited, overrode *environment.Environment, baseDir, filename string, content []byte) (*bytes.Buffer, error) {
	// try a first pass render. This will always succeed, but can produce a limited env
	if r.logger != nil {
		r.logger.Debugf("first-pass rendering starting for \"%s\": inherited=%v, overrode=%v", filename, inherited, overrode)
	}

	initEnv, err := inherited.Merge(overrode)
	if err != nil {
		return nil, err
	}

	if r.logger != nil {
		r.logger.Debugf("first-pass uses: %v", initEnv)
	}

	renderedEnv, prestate := r.renderPrestate(initEnv, baseDir, filename, content)

	if r.logger != nil {
		r.logger.Debugf("first-pass produced: %v", renderedEnv)
	}

	finalEnv, err := inherited.Merge(renderedEnv)
	if err != nil {
		return nil, err
	}

	finalEnv, err = finalEnv.Merge(overrode)
	if err != nil {
		return nil, err
	}

	if r.logger != nil {
		r.logger.Debugf("first-pass rendering result of \"%s\": %v", filename, *finalEnv)
	}

	vals, err := finalEnv.GetMergedValues()
	if err != nil {
		return nil, err
	}

	if prestate != nil {
		prestate.Env = *finalEnv
		r.logger.Debugf("vals:\n%v\ndefaultVals:%v", vals, prestate.DefaultValues)
	}

	tmplData := state.NewEnvironmentTemplateData(*finalEnv, r.namespace, vals)
	secondPassRenderer := tmpl.NewFileRenderer(r.fs, baseDir, tmplData)
	yamlBuf, err := secondPassRenderer.RenderTemplateContentToBuffer(content)
	if err != nil {
		if r.logger != nil {
			r.logger.Debugf("second-pass rendering failed, input of \"%s\":\n%s", filename, prependLineNumbers(string(content)))
		}
		return nil, err
	}
	if r.logger != nil {
		r.logger.Debugf("second-pass rendering result of \"%s\":\n%s", filename, prependLineNumbers(yamlBuf.String()))
	}
	return yamlBuf, nil
}
