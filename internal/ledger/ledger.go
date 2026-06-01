// Package ledger provides an append-only, in-memory journal of monetary
// entries keyed by account ID.
//
// The ledger is intentionally independent of the accounts package: the
// accountID is treated as an opaque string. Future iterations may wire
// ledger entries into account balance updates, but for now the ledger
// stands alone.
//
// All operations are safe for concurrent use.
package ledger

import (
	"fmt"
	"sync"
	"time"
)

// Entry is a single immutable monetary entry in the ledger.
//
// Amount is expressed in the smallest currency unit (e.g. cents) so that
// arithmetic is exact. Negative amounts represent debits.
type Entry struct {
	ID        string
	AccountID string
	Amount    int64
	CreatedAt time.Time
}

// Ledger is an append-only, in-memory journal of Entry values grouped by
// account ID. The zero value is not usable; construct a Ledger with New.
type Ledger struct {
	mu      sync.Mutex
	entries map[string][]Entry
	nextID  uint64
	now     func() time.Time // injectable for tests; defaults to time.Now
}

// New returns a Ledger ready for use.
func New() *Ledger {
	return &Ledger{
		entries: make(map[string][]Entry),
		now:     time.Now,
	}
}

// Append records a new Entry for accountID with the given amount and
// returns the persisted Entry. A fresh ID and CreatedAt timestamp are
// assigned by the ledger.
//
// Append is safe to call from multiple goroutines concurrently; each
// successful call produces a unique ID.
func (l *Ledger) Append(accountID string, amount int64) (Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.nextID++
	e := Entry{
		ID:        fmt.Sprintf("entry-%d", l.nextID),
		AccountID: accountID,
		Amount:    amount,
		CreatedAt: l.now(),
	}
	l.entries[accountID] = append(l.entries[accountID], e)
	return e, nil
}

// List returns the entries recorded for accountID in append order.
//
// A copy of the slice is returned so callers cannot mutate the ledger's
// internal storage. An unknown accountID yields an empty (non-nil) slice
// rather than nil or an error.
func (l *Ledger) List(accountID string) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	src := l.entries[accountID]
	out := make([]Entry, len(src))
	copy(out, src)
	return out
}
