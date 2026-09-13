package helmexec

import (
	"context"
	"io"
)

type HelmContext struct {
	HistoryMax  int
	WorkerIndex int
	Writer      io.Writer

	// Ctx, when set, carries the per-release span context so that helm
	// subprocesses started for this release nest under the release span
	// (consumed by the execer's execWithContext funnel; nil falls back to the
	// runner's own context, i.e. the historical behavior). Cancellation
	// semantics are unchanged either way: callers derive it from the app
	// context, which is where the runner's context comes from too.
	Ctx context.Context
}
