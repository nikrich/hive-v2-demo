// Package health is a thin handler adapter that exposes the
// internal/health.HealthHandler under the canonical
// internal/handlers/<endpoint> directory tree.
//
// Every endpoint in bank-go has a sibling package under
// internal/handlers/ so the server-wiring code can import handlers
// uniformly. Health is the trivial case: the adapter just re-exports the
// underlying HealthHandler — no extra middleware, validation, or logic.
package health

import (
	"net/http"

	healthcore "github.com/nikrich/hive-v2-demo/internal/health"
)

// HealthAdapter returns an http.HandlerFunc that delegates to
// internal/health.HealthHandler. It exists so callers can wire endpoints
// through the internal/handlers/ tree without depending on the core
// liveness package directly.
func HealthAdapter() http.HandlerFunc {
	return http.HandlerFunc(healthcore.HealthHandler)
}
