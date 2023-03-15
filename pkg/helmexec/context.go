package helmexec

import (
	"io"
)

type HelmContext struct {
	HistoryMax  int
	WorkerIndex int
	Writer      io.Writer
}
