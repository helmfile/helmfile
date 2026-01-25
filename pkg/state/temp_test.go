package state

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGenerateID(t *testing.T) {
	type testcase struct {
		subject string
		release ReleaseSpec
		data    any
		want    string
	}

	ids := map[string]int{}

	run := func(tc testcase) {
		t.Helper()

		t.Run(tc.subject, func(t *testing.T) {
			t.Helper()

			got, err := generateValuesID(&tc.release, tc.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if d := cmp.Diff(tc.want, got); d != "" {
				t.Fatalf("unexpected result: want (-), got (+):\n%s", d)
			}

			ids[got]++
		})
	}

	run(testcase{
		subject: "baseline",
		release: ReleaseSpec{Name: "foo", Chart: "incubator/raw"},
		want:    "foo-values-6b9c9f74fc",
	})

	run(testcase{
		subject: "different bytes content",
		release: ReleaseSpec{Name: "foo", Chart: "incubator/raw"},
		data:    []byte(`{"k":"v"}`),
		want:    "foo-values-6fdbdc99db",
	})

	run(testcase{
		subject: "different map content",
		release: ReleaseSpec{Name: "foo", Chart: "incubator/raw"},
		data:    map[string]any{"k": "v"},
		want:    "foo-values-659d694dc6",
	})

	run(testcase{
		subject: "different chart",
		release: ReleaseSpec{Name: "foo", Chart: "stable/envoy"},
		want:    "foo-values-67df747bb8",
	})

	run(testcase{
		subject: "different name",
		release: ReleaseSpec{Name: "bar", Chart: "incubator/raw"},
		want:    "bar-values-5f4bb5cf9d",
	})

	run(testcase{
		subject: "specific ns",
		release: ReleaseSpec{Name: "foo", Chart: "incubator/raw", Namespace: "myns"},
		want:    "myns-foo-values-5bb44469d8",
	})

	for id, n := range ids {
		if n > 1 {
			t.Fatalf("too many occurrences of %s: %d", id, n)
		}
	}
}
