package state

import "testing"

func TestAppendSkipSchemaValidationFlagToChartifyTemplateArgs(t *testing.T) {
	enable := true

	tests := []struct {
		name         string
		defaults     HelmSpec
		release      *ReleaseSpec
		templateArgs string
		want         string
	}{
		{
			name: "adds flag from release setting",
			release: &ReleaseSpec{
				SkipSchemaValidation: &enable,
			},
			want: "--skip-schema-validation",
		},
		{
			name: "adds flag from helm defaults",
			defaults: HelmSpec{
				SkipSchemaValidation: &enable,
			},
			release: &ReleaseSpec{},
			want:    "--skip-schema-validation",
		},
		{
			name: "appends flag to existing args",
			release: &ReleaseSpec{
				SkipSchemaValidation: &enable,
			},
			templateArgs: "--kube-context default",
			want:         "--kube-context default --skip-schema-validation",
		},
		{
			name: "does not duplicate existing flag",
			release: &ReleaseSpec{
				SkipSchemaValidation: &enable,
			},
			templateArgs: "--skip-schema-validation --kube-context default",
			want:         "--skip-schema-validation --kube-context default",
		},
		{
			name:         "does not add flag when disabled",
			release:      &ReleaseSpec{},
			templateArgs: "--kube-context default",
			want:         "--kube-context default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					HelmDefaults: tt.defaults,
				},
			}

			got := st.appendSkipSchemaValidationFlagToChartifyTemplateArgs(tt.templateArgs, tt.release)
			if got != tt.want {
				t.Fatalf("appendSkipSchemaValidationFlagToChartifyTemplateArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}
