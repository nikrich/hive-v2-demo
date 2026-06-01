package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nikrich/hive-v2-demo/internal/accounts"
	"github.com/nikrich/hive-v2-demo/internal/ledger"
	"github.com/nikrich/hive-v2-demo/internal/logger"
)

// newTestServer builds a Server with real dependencies (in-memory store,
// fresh ledger, logger writing to a discarded buffer) and returns it wrapped
// in an httptest.NewServer. The caller is responsible for ts.Close().
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := New(
		accounts.NewInMemoryStore(),
		ledger.New(),
		logger.New(&bytes.Buffer{}),
	)
	return httptest.NewServer(s.Handler())
}

// doRequest issues a request of method to ts.URL+path and returns the
// response. It fails the test on transport errors.
func doRequest(t *testing.T, ts *httptest.Server, method, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, nil)
	if err != nil {
		t.Fatalf("build request %s %s: %v", method, path, err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, path, err)
	}
	return resp
}

// TestHealthz_Returns200WithExpectedBody confirms the health route is wired
// to the real handler rather than a stub.
func TestHealthz_Returns200WithExpectedBody(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := doRequest(t, ts, http.MethodGet, "/healthz")
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Errorf("status: got %d, want %d", got, want)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if got, want := string(body), `{"status":"ok","service":"bank"}`; got != want {
		t.Errorf("body: got %q, want %q", got, want)
	}
}

// TestStubRoutes_Return501 covers every non-health endpoint registered by
// the server. Each must return HTTP 501 in this iteration; later stories
// will flip these to real status codes as handlers are implemented.
func TestStubRoutes_Return501(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"create account", http.MethodPost, "/accounts"},
		{"get account", http.MethodGet, "/accounts/abc123"},
		{"transfer", http.MethodPost, "/accounts/abc123/transfers"},
		{"balance", http.MethodGet, "/accounts/abc123/balance"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doRequest(t, ts, tc.method, tc.path)
			defer resp.Body.Close()

			if got, want := resp.StatusCode, http.StatusNotImplemented; got != want {
				t.Errorf("%s %s status: got %d, want %d", tc.method, tc.path, got, want)
			}
		})
	}
}

// TestEveryResponseCarriesRequestID proves the middleware wrap is in place
// by hitting one route per registered pattern (including the real health
// handler) and asserting X-Request-Id is non-empty on each response.
func TestEveryResponseCarriesRequestID(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/healthz"},
		{http.MethodPost, "/accounts"},
		{http.MethodGet, "/accounts/abc123"},
		{http.MethodPost, "/accounts/abc123/transfers"},
		{http.MethodGet, "/accounts/abc123/balance"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			resp := doRequest(t, ts, tc.method, tc.path)
			defer resp.Body.Close()

			if id := resp.Header.Get("X-Request-Id"); id == "" {
				t.Errorf("%s %s: X-Request-Id header missing or empty", tc.method, tc.path)
			}
		})
	}
}
