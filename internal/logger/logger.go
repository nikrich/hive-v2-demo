// Package logger provides a deliberately minimal structured JSON logger.
//
// The logger writes one JSON object per call to a caller-supplied io.Writer,
// which makes it trivial to capture output in tests without redirecting
// os.Stdout. Only the Info level is implemented; level filtering, formatting
// helpers, and sibling-package wiring are intentionally out of scope for this
// iteration.
package logger

import (
	"encoding/json"
	"io"
)

// Logger writes single-line JSON log records to an injected io.Writer.
//
// A Logger is safe to construct via New; the zero value is not usable because
// it has no writer.
type Logger struct {
	out io.Writer
}

// New returns a Logger that writes to out.
func New(out io.Writer) *Logger {
	return &Logger{out: out}
}

// logRecord is the on-the-wire shape of a single log line.
//
// Fields is always emitted (even when nil) so consumers can rely on its
// presence; encoding/json renders a nil map as JSON null, which decodes back
// to a nil map and still satisfies "carries the passed map".
type logRecord struct {
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields"`
}

// Info writes a single-line JSON record at level "info" to the configured
// writer. The fields map is passed through verbatim.
//
// A trailing newline is appended so concatenated output remains one record
// per line. Errors from the underlying writer are intentionally ignored —
// logging must never break the caller, and there is no sensible recovery
// path at this layer.
func (l *Logger) Info(msg string, fields map[string]any) {
	rec := logRecord{
		Level:   "info",
		Message: msg,
		Fields:  fields,
	}
	// json.Marshal of this struct cannot fail for supported field types;
	// if a caller passes an unmarshalable value inside fields we drop the
	// line rather than panic.
	data, err := json.Marshal(rec)
	if err != nil {
		return
	}
	data = append(data, '\n')
	_, _ = l.out.Write(data)
}
