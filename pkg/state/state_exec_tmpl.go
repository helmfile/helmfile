package state

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"dario.cat/mergo"

	"github.com/helmfile/helmfile/pkg/tmpl"
	"github.com/helmfile/helmfile/pkg/yaml"
)

func (st *HelmState) Values() map[string]any {
	if st.RenderedValues == nil {
		panic("[bug] RenderedValues is nil")
	}

	return st.RenderedValues
}

func (st *HelmState) createReleaseTemplateData(release *ReleaseSpec, vals map[string]any) releaseTemplateData {
	tmplData := releaseTemplateData{
		Environment: st.Env,
		KubeContext: st.OverrideKubeContext,
		Namespace:   st.OverrideNamespace,
		Chart:       st.OverrideChart,
		Values:      vals,
		Release: releaseTemplateDataRelease{
			Name:         release.Name,
			Chart:        release.Chart,
			Namespace:    release.Namespace,
			Labels:       release.Labels,
			KubeContext:  release.KubeContext,
			ChartVersion: release.Version,
		},
	}
	tmplData.StateValues = &tmplData.Values
	return tmplData
}

func getBoolRefFromStringTemplate(templateRef string) (*bool, error) {
	var result bool
	if err := yaml.Unmarshal([]byte(templateRef), &result); err != nil {
		return nil, fmt.Errorf("failed deserialising string %s: %v", templateRef, err)
	}
	return &result, nil
}


func updateBoolTemplatedValues(r *ReleaseSpec) error {
	if r.InstalledTemplate != nil {
		if installed, err := getBoolRefFromStringTemplate(*r.InstalledTemplate); err != nil {
			return fmt.Errorf("installedTemplate: %v", err)
		} else {
			r.InstalledTemplate = nil
			r.Installed = installed
		}
	}

	if r.ConditionTemplate != nil {
		if condition, err := getBoolRefFromStringTemplate(*r.ConditionTemplate); err != nil {
			return fmt.Errorf("conditionTemplate: %v", err)
		} else {
			r.ConditionTemplate = nil
			setConditionFromBool(r, *condition)
		}
	}

	if r.WaitTemplate != nil {
		if wait, err := getBoolRefFromStringTemplate(*r.WaitTemplate); err != nil {
			return fmt.Errorf("waitTemplate: %v", err)
		} else {
			r.WaitTemplate = nil
			r.Wait = wait
		}
	}

	if r.VerifyTemplate != nil {
		if verify, err := getBoolRefFromStringTemplate(*r.VerifyTemplate); err != nil {
			return fmt.Errorf("verifyTemplate: %v", err)
		} else {
			r.VerifyTemplate = nil
			r.Verify = verify
		}
	}

	return nil
}

func (st *HelmState) ExecuteTemplates() (*HelmState, error) {
	r := *st

	vals := st.Values()

	for i, rt := range st.Releases {
		rtWithDefaults := rt
		rtWithDefaults.Inherit = st.applyDefaultInherit(rt.Inherit)

		release, err := st.releaseWithInheritedTemplate(&rtWithDefaults, nil)
		if err != nil {
			var cyclicInheritanceErr CyclicReleaseTemplateInheritanceError
			if errors.As(err, &cyclicInheritanceErr) {
				return nil, fmt.Errorf("unable to load release %q with template: %w", rt.Name, cyclicInheritanceErr)
			}
			return nil, err
		}

		if release.KubeContext == "" {
			release.KubeContext = r.HelmDefaults.KubeContext
		}
		if release.Labels == nil {
			release.Labels = map[string]string{}
		}
		for k, v := range st.CommonLabels {
			release.Labels[k] = v
		}
		if len(release.ApiVersions) == 0 {
			release.ApiVersions = st.ApiVersions
		}
		if release.KubeVersion == "" {
			release.KubeVersion = st.KubeVersion
		}

		successFlag := false
		for it, prev := 0, release; it < 6; it++ {
			tmplData := st.createReleaseTemplateData(prev, vals)
			renderer := tmpl.NewFileRenderer(st.fs, st.basePath, tmplData)
			r, err := release.ExecuteTemplateExpressions(renderer)
			if err != nil {
				return nil, fmt.Errorf("failed executing templates in release \"%s\".\"%s\": %v", st.FilePath, release.Name, err)
			}
			if reflect.DeepEqual(prev, r) {
				successFlag = true
				if err := updateBoolTemplatedValues(r); err != nil {
					return nil, fmt.Errorf("failed executing templates in release \"%s\".\"%s\": %v", st.FilePath, release.Name, err)
				}
				st.Releases[i] = *r
				break
			}
			prev = r
		}
		if !successFlag {
			return nil, fmt.Errorf("failed executing templates in release \"%s\".\"%s\": %s", st.FilePath, release.Name,
				"recursive references can't be resolved")
		}

		if st.Releases[i].Chart == "" {
			return nil, fmt.Errorf("encountered empty chart while reading release %q", st.Releases[i].Name)
		}
	}

	return &r, nil
}

type CyclicReleaseTemplateInheritanceError struct {
	Message string
}

func (e CyclicReleaseTemplateInheritanceError) Error() string {
	return e.Message
}

// releaseWithInheritedTemplate generates a new ReleaseSpec from a ReleaseSpec, by recursively inheriting
// release templates referenced by the spec's `inherit` field.
// The third parameter retains the current state of the recursive call, to detect a cyclic dependency a.k.a
// a cyclic relese template inheritance.
// This functions fails with a CyclicReleaseTemplateInheritanceError if it finds a cyclic inheritance.
func (st *HelmState) releaseWithInheritedTemplate(r *ReleaseSpec, inheritancePath []string) (*ReleaseSpec, error) {
	var merged ReleaseSpec

	for _, inherit := range r.Inherit {
		templateName := inherit.Template
		if templateName == "" {
			return r, nil
		}

		path := append([]string{}, inheritancePath...)
		path = append(path, templateName)

		var cycleFound bool
		for _, t := range inheritancePath {
			if t == templateName {
				cycleFound = true
				break
			}
		}

		if cycleFound {
			return nil, CyclicReleaseTemplateInheritanceError{Message: fmt.Sprintf("cyclic inheritance detected: %s", strings.Join(path, "->"))}
		}

		template, defined := st.Templates[templateName]
		if !defined {
			return nil, fmt.Errorf("release %q tried to inherit inexistent release template %q", r.Name, templateName)
		}

		src, err := st.releaseWithInheritedTemplate(&template.ReleaseSpec, path)
		if err != nil {
			return nil, fmt.Errorf("unable to load release template %q: %w", templateName, err)
		}

		for _, k := range inherit.Except {
			switch k {
			case "labels":
				src.Labels = map[string]string{}
			case "values":
				src.Values = nil
			case "valuesTemplate":
				src.ValuesTemplate = nil
			case "setTemplate":
				src.SetValuesTemplate = nil
			case "set":
				src.SetValues = nil
			case "setString":
				src.SetStringValues = nil
			case "secrets":
				src.Secrets = nil
			default:
				return nil, fmt.Errorf("%q is not allowed under `inherit`. Allowed values are \"set\", \"setTemplate\", \"values\", \"valuesTemplate\", \"secrets\", and \"labels\"", k)
			}

			st.logger.Debugf("excluded field %q when inheriting template %q to release %q", k, templateName, r.Name)
		}

		if err := mergo.Merge(&merged, src, mergo.WithAppendSlice, mergo.WithSliceDeepCopy); err != nil {
			return nil, fmt.Errorf("unable to inherit release template %q: %w", templateName, err)
		}
	}

	if err := mergo.Merge(&merged, r, mergo.WithAppendSlice, mergo.WithSliceDeepCopy); err != nil {
		return nil, fmt.Errorf("unable to load release %q: %w", r.Name, err)
	}

	merged.Inherit = nil

	return &merged, nil
}

// applyDefaultInherit prepends default inherit templates to the release's inherit list.
// Templates that are already explicitly referenced by the release are not duplicated.
func (st *HelmState) applyDefaultInherit(releaseInherit Inherits) Inherits {
	if len(st.DefaultInherit) == 0 {
		return releaseInherit
	}

	// Build the deduplication set and filter out blank entries in one pass.
	existing := make(map[string]bool, len(releaseInherit))
	filtered := make(Inherits, 0, len(releaseInherit))
	for _, inh := range releaseInherit {
		if name := strings.TrimSpace(inh.Template); name != "" {
			existing[name] = true
			filtered = append(filtered, inh)
		}
	}

	result := make(Inherits, 0, len(st.DefaultInherit)+len(filtered))
	for _, name := range st.DefaultInherit {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		if !existing[name] {
			result = append(result, Inherit{Template: name})
			existing[name] = true
		}
	}
	result = append(result, filtered...)
	return result
}
