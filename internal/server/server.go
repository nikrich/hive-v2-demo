// Package server wires the bank-go HTTP surface area: routes, middleware, and
// the dependency hand-off from main into per-resource handlers.
//
// This iteration deliberately keeps the routing layer thin. Only the liveness
// endpoint is hooked up to a real handler (health.HealthHandler); every other
// route returns HTTP 501 Not Implemented as a placeholder. Subsequent stories
// will replace those stubs with real account, transfer, and balance handlers
// without touching this file's structure, because each handler lives in its
// own package and is registered here by name.
//
// All routes are wrapped with middleware.RequestID so every response carries
// an X-Request-Id header — the marker downstream stories will rely on for
// log correlation.
package server

import (
	"net/http"

	"github.com/nikrich/hive-v2-demo/internal/accounts"
	"github.com/nikrich/hive-v2-demo/internal/health"
	"github.com/nikrich/hive-v2-demo/internal/ledger"
	"github.com/nikrich/hive-v2-demo/internal/logger"
	"github.com/nikrich/hive-v2-demo/internal/middleware"
)

// Server is the central HTTP composition root. It owns the *http.ServeMux on
// which every route is registered and the long-lived dependencies that future
// handler packages will need.
//
// The struct is intentionally small: it does not implement http.Handler
// directly. Callers obtain a ready-to-serve handler via Handler(), which
// applies the standard middleware chain. Keeping the mux exported as a field
// would let callers register routes after construction and bypass that
// wrapping, so it is unexported.
type Server struct {
	mux      *http.ServeMux
	accounts accounts.Store
	ledger   *ledger.Ledger
	logger   *logger.Logger
}

// New constructs a Server with its routes already registered.
//
// Dependencies are accepted by interface (accounts.Store) or by pointer to a
// concrete type (*ledger.Ledger, *logger.Logger). They are stashed on the
// Server so the 501 stubs can be replaced with real handlers in later stories
// without changing this constructor's signature.
func New(accountsStore accounts.Store, ledgerSvc *ledger.Ledger, log *logger.Logger) *Server {
	s := &Server{
		mux:      http.NewServeMux(),
		accounts: accountsStore,
		ledger:   ledgerSvc,
		logger:   log,
	}
	s.registerRoutes()
	return s
}

// Handler returns the fully wrapped http.Handler suitable for passing to
// http.Server.Handler or httptest.NewServer.
//
// Today the chain is just RequestID. Additional middleware (panic recovery,
// access logging) can be layered here without touching individual route
// registrations.
func (s *Server) Handler() http.Handler {
	return middleware.RequestID(s.mux)
}

// registerRoutes installs every route the service exposes.
//
// Go 1.22's enhanced ServeMux pattern syntax is used so {id} path parameters
// can be parsed by future handlers via r.PathValue("id"). For this iteration
// the four non-health routes ignore the parameter and return 501.
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /healthz", health.HealthHandler)

	s.mux.HandleFunc("POST /accounts", notImplementedHandler)
	s.mux.HandleFunc("GET /accounts/{id}", notImplementedHandler)
	s.mux.HandleFunc("POST /accounts/{id}/transfers", notImplementedHandler)
	s.mux.HandleFunc("GET /accounts/{id}/balance", notImplementedHandler)
}

// notImplementedHandler is the shared placeholder for routes whose business
// logic is owned by a sibling story. It writes HTTP 501 with a minimal JSON
// body so callers (and tests) can distinguish a stub from a generic 404.
//
// Once a real handler replaces a route in registerRoutes, this function does
// not need to be touched.
func notImplementedHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_, _ = w.Write([]byte(`{"error":"not implemented"}`))
}
