package state

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSelectReleasesWithOverrides(t *testing.T) {
	type testcase struct {
		subject  string
		selector []string
		want     []string
	}

	testcases := []testcase{
		{
			subject:  "multiple OR selectors (nillable label first)",
			selector: []string{"type=bar", "name=nolabel2", "name=nolabel1"},
			want:     []string{"nolabel1", "nolabel2", "foo"},
		},
		{
			subject:  "multiple OR selectors (non-nillable label first)",
			selector: []string{"name=foo", "type!=bar"},
			want:     []string{"nolabel1", "nolabel2", "foo", "bar", "foobar"},
		},
		{
			subject:  "multiple AND conditions (nillable label first)",
			selector: []string{"type!=bar,name!=nolabel2"},
			want:     []string{"nolabel1", "bar", "foobar"},
		},
		{
			subject:  "multiple AND conditions (non-nillable label first)",
			selector: []string{"name!=nolabel2,type!=bar"},
			want:     []string{"nolabel1", "bar", "foobar"},
		},
		{
			subject:  "inequality on nillable label",
			selector: []string{"type!=bar"},
			want:     []string{"nolabel1", "nolabel2", "bar", "foobar"},
		},
		{
			subject:  "equality on nillable label",
			selector: []string{"type=bar"},
			want:     []string{"foo"},
		},
		{
			subject:  "inequality on non-nillable label",
			selector: []string{"name!=nolabel1"},
			want:     []string{"nolabel2", "foo", "bar", "foobar"},
		},
		{
			subject:  "equality on non-nillable label",
			selector: []string{"name=nolabel1"},
			want:     []string{"nolabel1"},
		},
		{
			subject:  "equality on non-alphabetic character label",
			selector: []string{"version=1.2/3"},
			want:     []string{"bar"},
		},
		{
			subject:  "equality on non-alphabetic character label",
			selector: []string{"version=1.2.3+123"},
			want:     []string{"foobar"},
		},
		{
			subject:  "inequality on non-alphatetic character label",
			selector: []string{"version!=1.2/3"},
			want:     []string{"nolabel1", "nolabel2", "foo", "foobar"},
		},
		{
			subject:  "inequality on non-alphatetic character label",
			selector: []string{"version!=1.2.3+123"},
			want:     []string{"nolabel1", "nolabel2", "foo", "bar"},
		},
	}

	example := []byte(`releases:
- name: nolabel1
  namespace: kube-system
  chart: stable/nolabel
- name: nolabel2
  namespace: default
  chart: stable/nolabel
- name: foo
  namespace: kube-system
  chart: stable/foo
  labels:
    type: bar
- name: bar
  namespace: kube-system
  chart: stable/bar
  labels:
    version: 1.2/3
- name: foobar
  namespace: kube-system
  chart: stable/bar
  labels:
    version: 1.2.3+123
`)

	state := stateTestEnv{
		Files: map[string]string{
			"/helmfile.yaml": string(example),
		},
		WorkDir: "/",
	}.MustLoadState(t, "/helmfile.yaml", "default")

	for _, tc := range testcases {
		var err error
		state.Selectors = tc.selector

		state.Releases, err = state.GetReleasesWithOverrides()
		if err != nil {
			t.Fatalf("%s %s: %v", tc.selector, tc.subject, err)
		}
		state.Releases = state.GetReleasesWithLabels()

		rs, err := state.GetSelectedReleases(false)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.selector, tc.subject, err)
		}

		var got []string

		for _, r := range rs {
			got = append(got, r.Name)
		}

		if d := cmp.Diff(tc.want, got); d != "" {
			t.Errorf("%s %s: %s", tc.selector, tc.subject, d)
		}
	}
}

func TestSelectReleasesWithOverridesWithIncludedTransitives(t *testing.T) {
	type testcase struct {
		subject                string
		selector               []string
		want                   []string
		includeTransitiveNeeds bool
	}

	testcases := []testcase{
		{
			subject:                "include transitives is false",
			selector:               []string{"name=serviceA"},
			want:                   []string{"serviceA"},
			includeTransitiveNeeds: false,
		},
		{
			subject:                "include transitives is true",
			selector:               []string{"name=serviceA"},
			want:                   []string{"serviceA", "serviceB", "serviceC"},
			includeTransitiveNeeds: true,
		},
	}

	example := []byte(`releases:
- name: serviceA
  namespace: default
  chart: stable/testchart
  needs:
    - serviceB
- name: serviceB
  namespace: default
  chart: stable/testchart
  needs:
    - serviceC
- name: serviceC
  namespace: default
  chart: stable/testchart
- name: serviceD
  namespace: default
  chart: stable/testchart
`)

	state := stateTestEnv{
		Files: map[string]string{
			"/helmfile.yaml": string(example),
		},
		WorkDir: "/",
	}.MustLoadState(t, "/helmfile.yaml", "default")

	for _, tc := range testcases {
		var err error
		state.Selectors = tc.selector
		state.Releases, err = state.GetReleasesWithOverrides()
		if err != nil {
			t.Fatalf("%s %s: %v", tc.selector, tc.subject, err)
		}
		state.Releases = state.GetReleasesWithLabels()

		rs, err := state.GetSelectedReleases(tc.includeTransitiveNeeds)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.selector, tc.subject, err)
		}

		var got []string

		for _, r := range rs {
			got = append(got, r.Name)
		}

		if d := cmp.Diff(tc.want, got); d != "" {
			t.Errorf("%s %s: %s", tc.selector, tc.subject, d)
		}
	}
}
