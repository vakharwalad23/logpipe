// Package logging produces the service's two log streams — application logs and
// HTTP access logs — as JSON, one object per line, using the standard library's
// log/slog. Each line carries a type field so the Vector pipeline can route it,
// and entries are written to a file on the shared volume.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

const defaultLogFile = "logs/app.log"

func NewAppLogger() (*slog.Logger, io.Closer, error) {
	path := os.Getenv("LOG_FILE")
	if path == "" {
		path = defaultLogFile
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create log dir for %q: %w", path, err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file %q: %w", path, err)
	}

	opts := &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		ReplaceAttr: renameForSchema,
	}
	logger := slog.New(slog.NewJSONHandler(f, opts))

	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	logger = logger.With(
		slog.String("log_type", "app"),
		slog.String("service", "crud-api"),
		slog.String("host", host),
	)

	return logger, f, nil
}

func renameForSchema(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 {
		switch a.Key {
		case slog.TimeKey:
			a.Key = "timestamp"
			a.Value = slog.TimeValue(a.Value.Time().UTC())
		case slog.MessageKey:
			a.Key = "message"
		}
	}
	return a
}
