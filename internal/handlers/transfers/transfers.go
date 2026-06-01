// Package transfers provides the HTTP handler for moving funds between two
// accounts.
//
// The handler is intentionally thin: it parses and validates the request,
// claims an idempotency key (if supplied), and writes a balanced pair of
// ledger entries (a debit on the source account and a matching credit on the
// destination account). It does not mutate account balances — balance
// derivation is a downstream concern of the ledger and out of scope for this
// iteration.
package transfers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/nikrich/hive-v2-demo/internal/accounts"
	"github.com/nikrich/hive-v2-demo/internal/idempotency"
	"github.com/nikrich/hive-v2-demo/internal/ledger"
)

// idempotencyTTL is how long a successful idempotency claim is honoured.
// 24h is generous for a demo and matches typical real-world API contracts
// (e.g. Stripe's Idempotency-Key behaviour).
const idempotencyTTL = 24 * time.Hour

// transferRequest is the decoded JSON request body.
type transferRequest struct {
	ToAccount string `json:"to_account"`
	Amount    int64  `json:"amount"`
}

// transferResponse is the JSON success response body.
type transferResponse struct {
	TransferID string `json:"transfer_id"`
	From       string `json:"from"`
	To         string `json:"to"`
	Amount     int64  `json:"amount"`
}

// Transfer returns an http.HandlerFunc that moves funds from the account
// identified by the {id} path value to the to_account named in the JSON
// body. When the request carries an Idempotency-Key header, a duplicate
// submission returns HTTP 200 with body "replayed" and writes no new ledger
// entries.
func Transfer(store accounts.Store, journal *ledger.Ledger, keys idempotency.KeyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fromID := r.PathValue("id")

		var req transferRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		// Validate request shape before touching anything stateful so that a
		// malformed retry doesn't burn the idempotency key.
		if req.ToAccount == "" {
			http.Error(w, "to_account is required", http.StatusBadRequest)
			return
		}
		if req.Amount <= 0 {
			http.Error(w, "amount must be positive", http.StatusBadRequest)
			return
		}

		// Verify both endpoints exist. We check the source first so the
		// caller gets a specific error per missing account.
		if _, err := store.GetAccount(fromID); err != nil {
			if errors.Is(err, accounts.ErrAccountNotFound) {
				http.Error(w, "from_account not found", http.StatusNotFound)
				return
			}
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}
		if _, err := store.GetAccount(req.ToAccount); err != nil {
			if errors.Is(err, accounts.ErrAccountNotFound) {
				http.Error(w, "to_account not found", http.StatusNotFound)
				return
			}
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}

		transferID, err := newTransferID()
		if err != nil {
			http.Error(w, "failed to generate transfer id", http.StatusInternalServerError)
			return
		}

		// Idempotency check. We stake the claim only after validation has
		// passed, so failed first attempts don't poison the key. The value
		// stored under the key is the transfer_id; it isn't returned on
		// replay (the contract requires literal body "replayed"), but it
		// makes the key store debuggable.
		if key := r.Header.Get("Idempotency-Key"); key != "" {
			won, _, err := keys.TrySet(key, transferID, idempotencyTTL)
			if err != nil {
				http.Error(w, "idempotency store error", http.StatusInternalServerError)
				return
			}
			if !won {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("replayed"))
				return
			}
		}

		// Write the balanced pair. The ledger is the source of truth; we
		// best-effort both legs in order. A real implementation would wrap
		// these in a transaction — see the package doc for the in-memory
		// caveat.
		if _, err := journal.Append(fromID, -req.Amount); err != nil {
			http.Error(w, "ledger append failed", http.StatusInternalServerError)
			return
		}
		if _, err := journal.Append(req.ToAccount, req.Amount); err != nil {
			http.Error(w, "ledger append failed", http.StatusInternalServerError)
			return
		}

		resp := transferResponse{
			TransferID: transferID,
			From:       fromID,
			To:         req.ToAccount,
			Amount:     req.Amount,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// newTransferID returns a fresh 8-character hex string (4 random bytes).
func newTransferID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
