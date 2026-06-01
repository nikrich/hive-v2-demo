// Package accounts provides HTTP handlers for the bank accounts resource.
//
// The handlers are intentionally decoupled from server wiring: they accept
// an accounts.Store and return http.HandlerFunc values that a later
// iteration will register against a mux. The package depends only on the
// internal/accounts leaf package.
package accounts

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/nikrich/hive-v2-demo/internal/accounts"
)

// createAccountRequest is the JSON body accepted by CreateAccount.
type createAccountRequest struct {
	Owner string `json:"owner"`
}

// CreateAccount returns an http.HandlerFunc that creates a new account.
//
// Request:  POST /accounts with body {"owner": "..."}
// Response: 201 Created with the new account as JSON on success
//
//	400 Bad Request when the body is missing, malformed, or the
//	owner field is empty
//	500 Internal Server Error when the store returns an unexpected
//	error
func CreateAccount(store accounts.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createAccountRequest
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if strings.TrimSpace(req.Owner) == "" {
			writeError(w, http.StatusBadRequest, "owner is required")
			return
		}

		acc, err := store.CreateAccount(req.Owner)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create account")
			return
		}

		writeJSON(w, http.StatusCreated, acc)
	}
}

// GetAccount returns an http.HandlerFunc that fetches an account by id.
//
// Request:  GET /accounts/{id}  (id read via r.PathValue, Go 1.22 routing)
// Response: 200 OK with the account as JSON on hit
//
//	400 Bad Request when the id path value is empty
//	404 Not Found when the account does not exist
//	500 Internal Server Error on unexpected store errors
func GetAccount(store accounts.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			writeError(w, http.StatusBadRequest, "id is required")
			return
		}

		acc, err := store.GetAccount(id)
		if err != nil {
			if errors.Is(err, accounts.ErrAccountNotFound) {
				writeError(w, http.StatusNotFound, "account not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to get account")
			return
		}

		writeJSON(w, http.StatusOK, acc)
	}
}

// writeJSON serializes v to w with the given status code and a JSON
// Content-Type header.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a uniform JSON error body {"error": "..."} with the
// given status code.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
