package health

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler_RespondsWithExpectedStatusHeaderAndBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	HealthHandler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Errorf("status code: got %d, want %d", got, want)
	}

	if got, want := resp.Header.Get("Content-Type"), "application/json"; got != want {
		t.Errorf("Content-Type header: got %q, want %q", got, want)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if got, want := string(bodyBytes), `{"status":"ok","service":"bank"}`; got != want {
		t.Errorf("body: got %q, want %q", got, want)
	}
}
