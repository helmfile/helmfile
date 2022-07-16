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
	w.log.Debugf("%s%s", w.prefix, strings.TrimSpace(string(p)))
	return len(p), nil
}
