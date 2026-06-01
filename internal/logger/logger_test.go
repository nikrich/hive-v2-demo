package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestInfo_WritesSingleJSONLine(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	fields := map[string]any{
		"user_id":    "u-123",
		"request_id": "r-456",
		"count":      float64(7), // float64 because JSON decode gives float64 for numbers
	}
	l.Info("hello world", fields)

	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected output to end with newline, got %q", out)
	}
	if strings.Count(out, "\n") != 1 {
		t.Fatalf("expected exactly one log line, got %d in %q", strings.Count(out, "\n"), out)
	}

	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimRight(out, "\n")), &rec); err != nil {
		t.Fatalf("output is not valid JSON: %v (raw=%q)", err, out)
	}

	if got, want := rec["message"], "hello world"; got != want {
		t.Errorf("message: got %v, want %v", got, want)
	}
	if got, want := rec["level"], "info"; got != want {
		t.Errorf("level: got %v, want %v", got, want)
	}

	gotFields, ok := rec["fields"].(map[string]any)
	if !ok {
		t.Fatalf("fields: expected object, got %T (%v)", rec["fields"], rec["fields"])
	}
	for k, want := range fields {
		if got := gotFields[k]; got != want {
			t.Errorf("fields[%q]: got %v (%T), want %v (%T)", k, got, got, want, want)
		}
	}
}

func TestInfo_NilFields(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	l.Info("no fields here", nil)

	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimRight(buf.String(), "\n")), &rec); err != nil {
		t.Fatalf("output is not valid JSON: %v (raw=%q)", err, buf.String())
	}

	if got, want := rec["message"], "no fields here"; got != want {
		t.Errorf("message: got %v, want %v", got, want)
	}
	if got, want := rec["level"], "info"; got != want {
		t.Errorf("level: got %v, want %v", got, want)
	}
	// Nil maps marshal to JSON null and decode back to nil — still satisfies
	// "fields field present, carrying the passed map".
	if _, present := rec["fields"]; !present {
		t.Errorf("fields key missing from output; want present (even if null)")
	}
	if rec["fields"] != nil {
		t.Errorf("fields: got %v, want nil for nil input map", rec["fields"])
	}
}

func TestInfo_MultipleCallsProduceOneLinePerCall(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	l.Info("first", map[string]any{"n": float64(1)})
	l.Info("second", map[string]any{"n": float64(2)})
	l.Info("third", map[string]any{"n": float64(3)})

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), buf.String())
	}
	wantMsgs := []string{"first", "second", "third"}
	for i, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %d not valid JSON: %v (raw=%q)", i, err, line)
		}
		if got := rec["message"]; got != wantMsgs[i] {
			t.Errorf("line %d message: got %v, want %v", i, got, wantMsgs[i])
		}
	}
}
