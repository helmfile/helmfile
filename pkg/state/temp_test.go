package state

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGenerateID(t *testing.T) {
	type testcase struct {
		subject string
		release ReleaseSpec
		data    interface{}
		want    string
	}

	ids := map[string]int{}

	run := func(tc testcase) {
		t.Helper()

		t.Run(tc.subject, func(t *testing.T) {
			t.Helper()

			got, err := generateValuesID(&tc.release, tc.data)
			if err != nil {
				t.Fatalf("uenxpected error: %v", err)
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
		want:    "foo-values-77cb4d7f9b",
	})

	run(testcase{
		subject: "different bytes content",
		release: ReleaseSpec{Name: "foo", Chart: "incubator/raw"},
		data:    []byte(`{"k":"v"}`),
		want:    "foo-values-6f6bc74b55",
	})

	run(testcase{
		subject: "different map content",
		release: ReleaseSpec{Name: "foo", Chart: "incubator/raw"},
		data:    map[string]interface{}{"k": "v"},
		want:    "foo-values-57dfc6cb8f",
	})

	run(testcase{
		subject: "different chart",
		release: ReleaseSpec{Name: "foo", Chart: "stable/envoy"},
		want:    "foo-values-74594bb67",
	})

	run(testcase{
		subject: "different name",
		release: ReleaseSpec{Name: "bar", Chart: "incubator/raw"},
		want:    "bar-values-bdbb54d7",
	})

	run(testcase{
		subject: "specific ns",
		release: ReleaseSpec{Name: "foo", Chart: "incubator/raw", Namespace: "myns"},
		want:    "myns-foo-values-677dcd477",
	})

	for id, n := range ids {
		if n > 1 {
			t.Fatalf("too many occurrences of %s: %d", id, n)
		}
	}
}
