// Package health provides a stateless HTTP liveness handler for bank-go.
//
// It is intentionally decoupled from server wiring; cmd/bank/main.go will
// register HealthHandler in a later iteration.
package health

import (
	"net/http"
)

// healthResponse is the exact JSON body served by HealthHandler.
//
// The literal byte form is fixed by the story's acceptance criteria
// ({"status":"ok","service":"bank"}), so we write it directly rather than
// round-tripping through encoding/json to avoid field-order drift.
const healthResponse = `{"status":"ok","service":"bank"}`

// HealthHandler reports liveness. It responds with HTTP 200, a JSON
// Content-Type, and a fixed body indicating the service is up.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(healthResponse))
}
