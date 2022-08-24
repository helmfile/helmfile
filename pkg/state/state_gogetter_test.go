package state

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

func TestGoGetter(t *testing.T) {
	logger := helmexec.NewLogger(os.Stderr, "warn")

	testcases := []struct {
		chart, dir string
		force      bool

		out, err string
	}{
		{
			chart: "raw/incubator",
			dir:   "",
			force: false,
			out:   "raw/incubator",
			err:   "",
		},
	}

	for i, tc := range testcases {
		test := tc
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			d, err := os.MkdirTemp("", "testgogetter")
			if err != nil {
				panic(err)
			}
			defer os.RemoveAll(d)

			st := &HelmState{
				logger:   logger,
				fs:       filesystem.DefaultFileSystem(),
				basePath: d,
			}

			out, err := st.goGetterChart(test.chart, test.dir, "", false)

			if diff := cmp.Diff(test.out, out); diff != "" {
				t.Fatalf("Unexpected out:\n%s", diff)
			}

			var errMsg string

			if err != nil {
				errMsg = err.Error()
			}

			if diff := cmp.Diff(test.err, errMsg); diff != "" {
				t.Fatalf("Unexpected err:\n%s", diff)
			}
		})
	}
}
