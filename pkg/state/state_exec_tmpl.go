package state

import (
	"fmt"
	"reflect"

	"gopkg.in/yaml.v3"

	"github.com/helmfile/helmfile/pkg/tmpl"
)

func (st *HelmState) Values() map[string]interface{} {
	if st.RenderedValues == nil {
		panic("[bug] RenderedValues is nil")
	}

	return st.RenderedValues
}

func (st *HelmState) createReleaseTemplateData(release *ReleaseSpec, vals map[string]interface{}) releaseTemplateData {
	tmplData := releaseTemplateData{
		Environment: st.Env,
		KubeContext: st.OverrideKubeContext,
		Namespace:   st.OverrideNamespace,
		Chart:       st.OverrideChart,
		Values:      vals,
		Release: releaseTemplateDataRelease{
			Name:        release.Name,
			Chart:       release.Chart,
			Namespace:   release.Namespace,
			Labels:      release.Labels,
			KubeContext: release.KubeContext,
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

	if r.WaitTemplate != nil {
		if wait, err := getBoolRefFromStringTemplate(*r.WaitTemplate); err != nil {
			return fmt.Errorf("waitTemplate: %v", err)
		} else {
			r.WaitTemplate = nil
			r.Wait = wait
		}
	}

	if r.TillerlessTemplate != nil {
		if tillerless, err := getBoolRefFromStringTemplate(*r.TillerlessTemplate); err != nil {
			return fmt.Errorf("tillerlessTemplate: %v", err)
		} else {
			r.TillerlessTemplate = nil
			r.Tillerless = tillerless
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
		release := rt
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
		for it, prev := 0, &release; it < 6; it++ {
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
	}

	return &r, nil
}
