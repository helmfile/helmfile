package state

import (
	"fmt"

	"github.com/helmfile/helmfile/pkg/maputil"
	"github.com/helmfile/helmfile/pkg/tmpl"
	"github.com/helmfile/helmfile/pkg/yaml"
)

func (r ReleaseSpec) ExecuteTemplateExpressions(renderer *tmpl.FileRenderer) (*ReleaseSpec, error) {
	var result *ReleaseSpec
	var err error

	result, err = r.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed executing template expressions in release \"%s\": %v", r.Name, err)
	}

	{
		ts := result.Name
		result.Name, err = renderer.RenderTemplateContentToString([]byte(ts))
		if err != nil {
			return nil, fmt.Errorf("failed executing template expressions in release \"%s\".name = \"%s\": %v", r.Name, ts, err)
		}
	}

	{
		ts := result.Chart
		result.Chart, err = renderer.RenderTemplateContentToString([]byte(ts))
		if err != nil {
			return nil, fmt.Errorf("failed executing template expressions in release \"%s\".chart = \"%s\": %v", r.Name, ts, err)
		}
	}

	{
		ts := result.Namespace
		result.Namespace, err = renderer.RenderTemplateContentToString([]byte(ts))
		if err != nil {
			return nil, fmt.Errorf("failed executing template expressions in release \"%s\".namespace = \"%s\": %v", r.Name, ts, err)
		}
	}

	{
		ts := result.Version
		result.Version, err = renderer.RenderTemplateContentToString([]byte(ts))
		if err != nil {
			return nil, fmt.Errorf("failed executing template expressions in release \"%s\".version = \"%s\": %v", r.Name, ts, err)
		}
	}

	if result.WaitTemplate != nil {
		ts := *result.WaitTemplate
		resultTmpl, err := renderer.RenderTemplateContentToString([]byte(ts))
		if err != nil {
			return nil, fmt.Errorf("failed executing template expressions in release \"%s\".version = \"%s\": %v", r.Name, ts, err)
		}
		result.WaitTemplate = &resultTmpl
	}

	if result.InstalledTemplate != nil {
		ts := *result.InstalledTemplate
		resultTmpl, err := renderer.RenderTemplateContentToString([]byte(ts))
		if err != nil {
			return nil, fmt.Errorf("failed executing template expressions in release \"%s\".version = \"%s\": %v", r.Name, ts, err)
		}
		result.InstalledTemplate = &resultTmpl
	}

	if result.VerifyTemplate != nil {
		ts := *result.VerifyTemplate
		resultTmpl, err := renderer.RenderTemplateContentToString([]byte(ts))
		if err != nil {
			return nil, fmt.Errorf("failed executing template expressions in release \"%s\".version = \"%s\": %v", r.Name, ts, err)
		}
		result.VerifyTemplate = &resultTmpl
	}

	for key, val := range result.Labels {
		ts := val
		s, err := renderer.RenderTemplateContentToBuffer([]byte(ts))
		if err != nil {
			return nil, fmt.Errorf("failed executing template expressions in release \"%s\".labels[%s] = \"%s\": %v", r.Name, key, ts, err)
		}
		result.Labels[key] = s.String()
	}

	if len(result.ValuesTemplate) > 0 {
		for i, t := range result.ValuesTemplate {
			switch ts := t.(type) {
			case map[interface{}]interface{}, map[string]interface{}:
				serialized, err := yaml.Marshal(ts)
				if err != nil {
					return nil, fmt.Errorf("failed executing template expressions in release \"%s\".values[%d] = \"%v\": %v", r.Name, i, ts, err)
				}

				s, err := renderer.RenderTemplateContentToBuffer(serialized)
				if err != nil {
					return nil, fmt.Errorf("failed executing template expressions in release \"%s\".values[%d] = \"%v\": %v", r.Name, i, string(serialized), err)
				}

				var deserialized map[string]interface{}

				if err := yaml.Unmarshal(s.Bytes(), &deserialized); err != nil {
					return nil, fmt.Errorf("failed executing template expressions in release \"%s\".values[%d] = \"%v\": %v", r.Name, i, ts, err)
				}

				m, err := maputil.CastKeysToStrings(deserialized)
				if err != nil {
					return nil, fmt.Errorf("failed executing template expressions in release \"%s\".values[%d] = \"%v\": %v", r.Name, i, ts, err)
				}

				result.ValuesTemplate[i] = m
			}
		}

		var newvals []interface{}
		newvals = append(newvals, result.ValuesTemplate...)
		result.Values = append(newvals, result.Values...)
	}

	for i, t := range result.Values {
		switch ts := t.(type) {
		case string:
			s, err := renderer.RenderTemplateContentToBuffer([]byte(ts))
			if err != nil {
				return nil, fmt.Errorf("failed executing template expressions in release \"%s\".values[%d] = \"%s\": %v", r.Name, i, ts, err)
			}
			result.Values[i] = s.String()
		case map[interface{}]interface{}:
			m, err := maputil.CastKeysToStrings(ts)
			if err != nil {
				return nil, fmt.Errorf("failed executing template expressions in release \"%s\".values[%d] = \"%s\": %v", r.Name, i, ts, err)
			}
			result.Values[i] = m
		}
	}

	for i, t := range result.Secrets {
		switch ts := t.(type) {
		case string:
			s, err := renderer.RenderTemplateContentToBuffer([]byte(ts))
			if err != nil {
				return nil, fmt.Errorf("failed executing template expressions in release \"%s\".secrets[%d] = \"%s\": %v", r.Name, i, ts, err)
			}
			result.Secrets[i] = s.String()
		}
	}

	if len(result.SetValuesTemplate) > 0 {
		for i, val := range result.SetValuesTemplate {
			{
				// name
				ts := val.Name
				s, err := renderer.RenderTemplateContentToBuffer([]byte(ts))
				if err != nil {
					return nil, fmt.Errorf("failed executing template expressions in release \"%s\".set[%d].name = \"%s\": %v", r.Name, i, ts, err)
				}
				result.SetValuesTemplate[i].Name = s.String()
			}
			{
				// value
				ts := val.Value
				s, err := renderer.RenderTemplateContentToBuffer([]byte(ts))
				if err != nil {
					return nil, fmt.Errorf("failed executing template expressions in release \"%s\".set[%d].value = \"%s\": %v", r.Name, i, ts, err)
				}
				result.SetValuesTemplate[i].Value = s.String()
			}
			{
				// file
				ts := val.File
				s, err := renderer.RenderTemplateContentToBuffer([]byte(ts))
				if err != nil {
					return nil, fmt.Errorf("failed executing template expressions in release \"%s\".set[%d].file = \"%s\": %v", r.Name, i, ts, err)
				}
				result.SetValuesTemplate[i].File = s.String()
			}
			for j, ts := range val.Values {
				// values
				s, err := renderer.RenderTemplateContentToBuffer([]byte(ts))
				if err != nil {
					return nil, fmt.Errorf("failed executing template expressions in release \"%s\".set[%d].values[%d] = \"%s\": %v", r.Name, i, j, ts, err)
				}
				result.SetValuesTemplate[i].Values[j] = s.String()
			}
		}

		var newvals []SetValue
		newvals = append(newvals, result.SetValuesTemplate...)
		result.SetValues = append(newvals, result.SetValues...)
	}

	for i, n := range result.Needs {
		s, err := renderer.RenderTemplateContentToBuffer([]byte(n))
		if err != nil {
			return nil, fmt.Errorf("failed executing template expressions in release \"%s\".needs[%d] = \"%s\": %v", r.Name, i, n, err)
		}
		result.Needs[i] = s.String()
	}

	return result, nil
}

func (r ReleaseSpec) Clone() (*ReleaseSpec, error) {
	serialized, err := yaml.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("failed cloning release \"%s\": %v", r.Name, err)
	}

	var deserialized ReleaseSpec

	if err := yaml.Unmarshal(serialized, &deserialized); err != nil {
		return nil, fmt.Errorf("failed cloning release \"%s\": %v", r.Name, err)
	}

	return &deserialized, nil
}

func (r ReleaseSpec) Desired() bool {
	return r.Installed == nil || *r.Installed
}
