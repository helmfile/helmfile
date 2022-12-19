package helmexec

import (
	"strings"

	"go.uber.org/zap"
)

type logWriterGenerator struct {
	log *zap.SugaredLogger
}

func (g logWriterGenerator) Writer(prefix string) *logWriter {
	return &logWriter{
		log:    g.log,
		prefix: prefix,
	}
}

type logWriter struct {
	log    *zap.SugaredLogger
	prefix string
}

func (w *logWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(string(p), "\n") {
		w.log.Debugf("%s%s", w.prefix, strings.TrimSpace(line))
	}
	return len(p), nil
}
