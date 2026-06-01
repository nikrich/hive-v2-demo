package middleware

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

// hexRE matches one or more lowercase hex characters. Combined with an
// explicit length check, this validates the full ID format.
var hexRE = regexp.MustCompile(`^[0-9a-f]+$`)

// TestRequestID_HeaderSet verifies that the middleware sets the X-Request-Id
// response header to an 8-character lowercase hex string.
func TestRequestID_HeaderSet(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	id := rec.Header().Get("X-Request-Id")
	if got, want := len(id), 8; got != want {
		t.Fatalf("X-Request-Id length: got %d (%q), want %d", got, id, want)
	}
	if !hexRE.MatchString(id) {
		t.Fatalf("X-Request-Id %q does not match [0-9a-f]+", id)
	}
}

// TestRequestID_ContextRoundTrip verifies that the same ID surfaced in the
// response header is also stored on the request context under
// CtxKeyRequestID, so downstream handlers can read it back.
func TestRequestID_ContextRoundTrip(t *testing.T) {
	var ctxID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, ok := r.Context().Value(CtxKeyRequestID).(string)
		if !ok {
			t.Fatalf("context value under CtxKeyRequestID is not a string: %#v", r.Context().Value(CtxKeyRequestID))
		}
		ctxID = v
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	headerID := rec.Header().Get("X-Request-Id")
	if headerID == "" {
		t.Fatal("X-Request-Id header was not set")
	}
	if ctxID != headerID {
		t.Fatalf("context ID %q does not match header ID %q", ctxID, headerID)
	}
}

// TestRequestID_PerRequestUnique sanity-checks that two consecutive requests
// receive distinct IDs. Not strictly in the acceptance criteria, but a cheap
// guard against an accidental constant-ID regression.
func TestRequestID_PerRequestUnique(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	rec1 := httptest.NewRecorder()
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/", nil))
	handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))

	id1 := rec1.Header().Get("X-Request-Id")
	id2 := rec2.Header().Get("X-Request-Id")
	if id1 == id2 {
		t.Fatalf("expected distinct request IDs, got %q twice", id1)
	}
}
