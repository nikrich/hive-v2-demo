// Package middleware contains small, composable net/http middlewares for the
// bank-go service. Each middleware is independent and has no cross-package
// imports from other internal/* siblings, so they can be developed and tested
// in isolation.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// ctxKey is an unexported type used for context keys defined in this package.
// Using a typed key (rather than a bare string) prevents accidental collisions
// with keys defined in other packages.
type ctxKey int

// CtxKeyRequestID is the context key under which the per-request correlation
// ID is stored. Handlers downstream of RequestID can retrieve the ID with:
//
//	id, _ := r.Context().Value(middleware.CtxKeyRequestID).(string)
const CtxKeyRequestID ctxKey = 0

// requestIDHeader is the HTTP header used to surface the correlation ID back
// to the client.
const requestIDHeader = "X-Request-Id"

// requestIDLength is the length, in hex characters, of the generated ID.
// 8 hex chars = 4 bytes of entropy, which is enough to make collisions
// unlikely within a single log window while staying short and human-readable.
const requestIDLength = 8

// newRequestID returns a random hex string of length requestIDLength.
// crypto/rand is used so IDs are unpredictable; this is correlation, not
// security, but using a CSPRNG avoids any temptation to seed math/rand.
func newRequestID() string {
	// requestIDLength hex chars = requestIDLength/2 bytes.
	buf := make([]byte, requestIDLength/2)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand.Read should never fail on supported platforms; if it
		// does, fall back to a fixed sentinel so we still produce a valid
		// 8-char hex string rather than panicking inside a middleware.
		return "00000000"
	}
	return hex.EncodeToString(buf)
}

// RequestID is an http.Handler middleware that assigns a short hex
// correlation ID to every incoming request. The ID is:
//
//  1. written to the response header X-Request-Id, and
//  2. injected into the request context under CtxKeyRequestID,
//
// so both the client and downstream handlers can correlate logs and traces
// for the same request.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), CtxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
