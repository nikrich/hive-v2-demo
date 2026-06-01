// Package accounts is the leaf domain package for bank accounts.
//
// It defines the Account value, the Store interface, and an in-memory
// implementation suitable for early development and tests. The interface is
// deliberately decoupled from any persistence concern so a later iteration
// can swap in a real store (Postgres, DynamoDB, ...) without touching
// callers.
//
// The package has no imports from sibling internal packages — Account stays
// local here for now.
package accounts

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
)

// Account is the canonical bank account value.
type Account struct {
	ID      string
	Owner   string
	Balance int64
}

// ErrAccountNotFound is returned by Store.GetAccount when the requested ID
// does not exist. Callers should compare with errors.Is.
var ErrAccountNotFound = errors.New("account not found")

// Store is the abstract persistence boundary for accounts. Implementations
// must be safe for concurrent use.
type Store interface {
	CreateAccount(owner string) (Account, error)
	GetAccount(id string) (Account, error)
}

// InMemoryStore is a goroutine-safe, in-process implementation of Store.
// Zero value is not usable; construct with NewInMemoryStore.
type InMemoryStore struct {
	mu       sync.Mutex
	accounts map[string]Account
}

// NewInMemoryStore returns a ready-to-use InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		accounts: make(map[string]Account),
	}
}

// CreateAccount creates a new account for the given owner with a freshly
// minted 8-character hex ID and a zero balance. It is safe to call from
// multiple goroutines; each call produces a distinct ID.
func (s *InMemoryStore) CreateAccount(owner string) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate an ID, retrying on the astronomically unlikely collision.
	var id string
	for {
		candidate, err := newAccountID()
		if err != nil {
			return Account{}, err
		}
		if _, exists := s.accounts[candidate]; !exists {
			id = candidate
			break
		}
	}

	acc := Account{
		ID:      id,
		Owner:   owner,
		Balance: 0,
	}
	s.accounts[id] = acc
	return acc, nil
}

// GetAccount returns the account with the given ID, or ErrAccountNotFound.
func (s *InMemoryStore) GetAccount(id string) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, ok := s.accounts[id]
	if !ok {
		return Account{}, ErrAccountNotFound
	}
	return acc, nil
}

// newAccountID returns a fresh 8-character hex string (4 random bytes).
func newAccountID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
