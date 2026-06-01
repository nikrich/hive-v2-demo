// Package balance provides the HTTP handler that reports an account's
// balance by summing every ledger entry recorded for that account.
//
// The handler is intentionally cache-free: balance is recomputed on every
// request as a straight sum across the ledger. This keeps the handler
// trivially correct and makes it easy to seed entries directly in tests.
//
// The package is wiring-agnostic — registration on a *http.ServeMux is
// performed by the main package in a later iteration.
package balance

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nikrich/hive-v2-demo/internal/accounts"
	"github.com/nikrich/hive-v2-demo/internal/ledger"
)

// response is the JSON payload returned by Balance on success.
//
// Field tags fix the wire format so callers (and tests) can rely on the
// exact key names regardless of struct field renames.
type response struct {
	AccountID string `json:"account_id"`
	Balance   int64  `json:"balance"`
}

// Balance returns an http.HandlerFunc that serves
//
//	GET /accounts/{id}/balance
//
// The account ID is read from r.PathValue("id") (Go 1.22 ServeMux pattern
// variables). If the account does not exist in store, the handler responds
// with HTTP 404. Otherwise it sums every entry in journal for that
// account_id (positive and negative amounts) and writes the result as
// JSON: {"account_id":"<id>","balance":<int64>}.
func Balance(store accounts.Store, journal *ledger.Ledger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		if _, err := store.GetAccount(id); err != nil {
			if errors.Is(err, accounts.ErrAccountNotFound) {
				http.Error(w, "account not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		var total int64
		for _, entry := range journal.List(id) {
			total += entry.Amount
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response{
			AccountID: id,
			Balance:   total,
		})
	}
}
