// Command bank is the HTTP server entrypoint for the bank-go service.
//
// It wires the long-lived dependencies (accounts store, ledger, logger) into
// the internal/server composition root and listens on :8080. The binary is
// intentionally thin: it owns lifecycle and process-level concerns only —
// everything routing-related lives in internal/server.
package main

import (
	"net/http"
	"os"

	"github.com/nikrich/hive-v2-demo/internal/accounts"
	"github.com/nikrich/hive-v2-demo/internal/ledger"
	"github.com/nikrich/hive-v2-demo/internal/logger"
	"github.com/nikrich/hive-v2-demo/internal/server"
)

// listenAddr is the TCP address the server binds to. Hard-coded for this
// iteration; a later story will read it from the environment.
const listenAddr = ":8080"

func main() {
	store := accounts.NewInMemoryStore()
	journal := ledger.New()
	log := logger.New(os.Stdout)

	srv := server.New(store, journal, log)

	log.Info("bank listening on "+listenAddr, map[string]any{
		"addr": listenAddr,
	})

	// http.ListenAndServe only returns on error; the error is logged and the
	// process exits non-zero so a supervisor (or `go run`) sees the failure.
	if err := http.ListenAndServe(listenAddr, srv.Handler()); err != nil {
		log.Info("bank server stopped", map[string]any{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}
