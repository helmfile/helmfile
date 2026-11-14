package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubhelmfileValuesTemplateUsesParentValues(t *testing.T) {
	for _, value := range []string{"from-parent", "from-cli", "child-default"} {
		t.Run(value, func(t *testing.T) {
			files := map[string]string{
				"/path/to/helmfile.yaml": `
environments:
  default:
    values:
    - foo: from-parent
      parentOnly: not-inherited
helmfiles:
- path: nested/child.yaml.gotmpl
  values:
  - overrides/values.yaml.gotmpl
  - overrides/plain.yaml
  - inline: preserved
`,
				"/path/to/overrides/values.yaml.gotmpl": `
provided: {{ .Values.foo | quote }}
environmentName: {{ .Environment.Name | quote }}
relative: {{ readFile "sibling.txt" | quote }}
`,
				"/path/to/overrides/sibling.txt": "beside-values-file",
				"/path/to/overrides/plain.yaml":  "plain: preserved\n",
				"/path/to/nested/child.yaml.gotmpl": `
values:
- provided: child-default
  childOnly: preserved
---
releases:
- name: {{ .Values.provided }}
  chart: example/child
`,
			}
			app := &App{
				OverrideHelmBinary:              DefaultHelmBinary,
				OverrideKubeContext:             "default",
				DisableKubeVersionAutoDetection: true,
				Env:                             "default",
				FileOrDir:                       "helmfile.yaml",
				Logger:                          newAppTestLogger(),
			}
			switch value {
			case "from-cli":
				app.Set = map[string]any{"foo": value}
			case "child-default":
				app.FileOrDir = "nested/child.yaml.gotmpl"
			}
			states := runSubhelmfileInheritsTest(t, files, app)
			child := states["child.yaml.gotmpl"]
			require.NotNil(t, child)
			require.Len(t, child.Releases, 1)
			assert.Equal(t, value, child.Releases[0].Name)
			if value != "child-default" {
				assert.Equal(t, "default", child.RenderedValues["environmentName"])
				assert.Equal(t, "beside-values-file", child.RenderedValues["relative"])
				assert.Equal(t, "preserved", child.RenderedValues["plain"])
				assert.Equal(t, "preserved", child.RenderedValues["inline"])
			}
			assert.Equal(t, "preserved", child.RenderedValues["childOnly"])
			assert.NotContains(t, child.RenderedValues, "parentOnly")
		})
	}
}
