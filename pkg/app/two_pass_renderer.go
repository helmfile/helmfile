package app

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/state"
	"github.com/helmfile/helmfile/pkg/tmpl"
)

func prependLineNumbers(text string) string {
	buf := bytes.NewBufferString("")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		fmt.Fprintf(buf, "%2d: %s\n", i, line)
	}
	return buf.String()
}

type RenderOpts struct {
}

func (r *desiredStateLoader) renderTemplatesToYaml(baseDir, filename string, content []byte) (*bytes.Buffer, error) {
	env := &environment.Environment{Name: r.env, Values: map[string]any(nil)}

	return r.renderTemplatesToYamlWithEnv(baseDir, filename, content, env, nil)
}

func (r *desiredStateLoader) renderTemplatesToYamlWithEnv(baseDir, filename string, content []byte, inherited, overrode *environment.Environment) (*bytes.Buffer, error) {
	return r.twoPassRenderTemplateToYaml(inherited, overrode, baseDir, filename, content)
}

func (r *desiredStateLoader) twoPassRenderTemplateToYaml(inherited, overrode *environment.Environment, baseDir, filename string, content []byte) (*bytes.Buffer, error) {
	var phase string
	r.logger.Debugf("%srendering starting for \"%s\": inherited=%v, overrode=%v", phase, filename, inherited, overrode)

	initEnv, err := inherited.Merge(nil)
	if err != nil {
		return nil, err
	}

	var (
		renderingPhase string
		finalEnv       *environment.Environment
		vals           map[string]any
	)

	finalEnv, err = initEnv.Merge(overrode)
	if err != nil {
		return nil, err
	}

	vals, err = finalEnv.GetMergedValues()
	if err != nil {
		return nil, err
	}

	tmplData := state.NewEnvironmentTemplateData(*finalEnv, r.namespace, vals)
	yamlBuf, err := r.renderWithSelectorFallback(content, baseDir, filename, tmplData)
	if err != nil {
		r.logger.Debugf("%srendering failed, input of \"%s\":\n%s", renderingPhase, filename, prependLineNumbers(string(content)))
		return nil, err
	}
	r.logger.Debugf("%srendering result of \"%s\":\n%s", renderingPhase, filename, prependLineNumbers(yamlBuf.String()))
	return yamlBuf, nil
}

// renderWithSelectorFallback renders content and, when selectors are active,
// retries with lenient requiredEnv if the first attempt fails due to a
// requiredEnv error. This lets the document parse so selector filtering can
// exclude releases whose env vars were not set.
// See https://github.com/helmfile/helmfile/issues/1172
func (r *desiredStateLoader) renderWithSelectorFallback(content []byte, baseDir, filename string, tmplData any) (*bytes.Buffer, error) {
	yamlBuf, err := tmpl.NewFileRenderer(r.fs, baseDir, tmplData).RenderTemplateContentToBuffer(content)
	if err == nil || len(r.selectors) == 0 || !isRequiredEnvError(err) {
		return yamlBuf, err
	}

	r.logger.Warnf(
		"rendering of \"%s\" failed due to a requiredEnv error; retrying with lenient requiredEnv "+
			"because selectors %v are active. See https://github.com/helmfile/helmfile/issues/1172",
		filename, r.selectors,
	)

	lenientRenderer := tmpl.NewLenientFileRenderer(r.fs, baseDir, tmplData)
	yamlBuf, err = lenientRenderer.RenderTemplateContentToBuffer(content)
	if err != nil {
		return nil, err
	}

	if missing := lenientRenderer.Context.GetMissingRequiredEnvs(); len(missing) > 0 {
		r.logger.Warnf("required env vars were unset/empty and rendered as empty strings in \"%s\": %v", filename, missing)
	}

	return yamlBuf, nil
}

func isRequiredEnvError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "required env var")
}
