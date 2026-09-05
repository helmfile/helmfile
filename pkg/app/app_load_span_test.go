package app

import (
	goContext "context"
	"os"
	"path/filepath"
	"testing"

	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/helmfile/helmfile/pkg/exectest"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/telemetry"
	"github.com/helmfile/helmfile/pkg/telemetry/otlptest"
)

// TestLoadSpanHierarchy drives a template run with telemetry enabled and the
// exectest fake helm, then asserts the state-loading span tree:
// root → discover_states → load → { render, parse }. This is the golden test
// for the app-layer context plumbing (docs/proposals/otel-tracing.md §10.3).
func TestLoadSpanHierarchy(t *testing.T) {
	rec := otlptest.NewRecorder(t)
	otlptest.SetupTelemetry(t, rec, "helmfile template")

	files := map[string]string{
		"/path/to/helmfile.yaml.gotmpl": `
releases:
- name: demo
  chart: incubator/raw
`,
	}

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	require.NoError(t, err)

	helm := &exectest.Helm{
		FailOnUnexpectedList: true,
		FailOnUnexpectedDiff: true,
	}

	app := appWithFs(&App{
		OverrideHelmBinary:              DefaultHelmBinary,
		fs:                              &ffs.FileSystem{Glob: filepath.Glob},
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		Logger:                          helmexec.NewLogger(os.Stderr, "warn"),
		helms: map[helmKey]helmexec.Interface{
			createHelmKey("helm", "default"): helm,
		},
		valsRuntime: valsRuntime,
		ctx:         telemetry.CommandContext(),
	}, files)

	err = app.Template(applyConfig{
		concurrency:            1,
		includeTransitiveNeeds: true,
		logger:                 app.Logger,
	})
	require.NoError(t, err)

	otlptest.ShutdownTelemetry(t)

	spans := rec.Spans(t)
	find := func(name string) *v1.Span {
		return otlptest.FindSpanWhere(t, spans, func(s *v1.Span) bool { return s.Name == name }, name)
	}

	root := find("helmfile template")
	discover := find("helmfile.discover_states")
	load := find("helmfile.load")
	render := find("helmfile.render")
	parse := find("helmfile.parse")

	// Every span joins the command trace started by SetupTelemetry.
	for _, s := range []*v1.Span{discover, load, render, parse} {
		assert.Equal(t, root.TraceId, s.TraceId, "%s must join the command trace", s.Name)
	}

	assert.Equal(t, root.SpanId, discover.ParentSpanId, "discover nests under root")
	assert.Equal(t, root.SpanId, load.ParentSpanId, "load nests under root")
	assert.Equal(t, load.SpanId, render.ParentSpanId, "render nests under load")
	assert.Equal(t, load.SpanId, parse.ParentSpanId, "parse nests under load")

	file, ok := otlptest.AttrString(load, "helmfile.state_file")
	require.True(t, ok)
	assert.Contains(t, file, "helmfile.yaml.gotmpl")
}

// TestLoadSpansAbsentWhenTelemetryDisabled pins that with telemetry disabled
// (the default) a template run behaves exactly as before and exports nothing.
func TestLoadSpansAbsentWhenTelemetryDisabled(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
releases:
- name: demo
  chart: incubator/raw
`,
	}

	helm := &exectest.Helm{}

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	require.NoError(t, err)

	app := appWithFs(&App{
		OverrideHelmBinary:              DefaultHelmBinary,
		fs:                              &ffs.FileSystem{Glob: filepath.Glob},
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		Logger:                          helmexec.NewLogger(os.Stderr, "warn"),
		helms: map[helmKey]helmexec.Interface{
			createHelmKey("helm", "default"): helm,
		},
		valsRuntime: valsRuntime,
		ctx:         goContext.Background(),
	}, files)

	tmplErr := app.Template(applyConfig{
		concurrency: 1,
		logger:      app.Logger,
	})
	require.NoError(t, tmplErr)
}
