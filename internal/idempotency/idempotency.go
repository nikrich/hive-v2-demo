// Package idempotency provides a small key-store abstraction used to make
// POST-style operations safe to retry: a caller stakes a claim on a key, and
// either learns "I won, my value was stored" or "someone got here first, here
// is their value" without performing a second lookup.
package idempotency

import (
	"sync"
	"time"
)

// KeyStore is the contract for an idempotency key store.
//
// TrySet attempts to associate value with key for ttl. Its return value is a
// (won, stored, err) triple:
//
//   - (true, value, nil)         — the key was absent (or expired) and value
//                                   was stored under it. "value" is the value
//                                   the caller passed in.
//   - (false, existing, nil)     — the key already had a live value; "existing"
//                                   is the value stored by the first writer
//                                   and remains untouched.
//   - (_, _, err)                — implementation-level error.
type KeyStore interface {
	TrySet(key string, value string, ttl time.Duration) (bool, string, error)
}

// Clock is the minimal time source used by InMemoryKeyStore. Production code
// uses the default wall clock; tests can inject a fake clock to make TTL
// expiry deterministic without sleeping.
type Clock func() time.Time

type entry struct {
	value     string
	expiresAt time.Time
}

// InMemoryKeyStore is a goroutine-safe, in-process implementation of KeyStore.
//
// Expiry is lazy: an entry is removed on the next TrySet that observes it as
// expired. There is no background sweeper, which keeps the type cheap and
// dependency-free; if memory pressure ever becomes a concern, a sweeper can
// be layered on without changing the public API.
type InMemoryKeyStore struct {
	mu      sync.Mutex
	entries map[string]entry
	now     Clock
}

// NewInMemoryKeyStore returns an empty store using the wall clock.
func NewInMemoryKeyStore() *InMemoryKeyStore {
	return NewInMemoryKeyStoreWithClock(time.Now)
}

// NewInMemoryKeyStoreWithClock returns an empty store using the supplied
// clock. Intended for tests that need to advance time without sleeping.
func NewInMemoryKeyStoreWithClock(now Clock) *InMemoryKeyStore {
	if now == nil {
		now = time.Now
	}
	return &InMemoryKeyStore{
		entries: make(map[string]entry),
		now:     now,
	}
}

// TrySet implements KeyStore.
func (s *InMemoryKeyStore) TrySet(key string, value string, ttl time.Duration) (bool, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if existing, ok := s.entries[key]; ok && existing.expiresAt.After(now) {
		// Live entry — caller lost the race.
		return false, existing.value, nil
	}

	// Either no entry, or the previous one expired. Lazy expiry: just overwrite.
	s.entries[key] = entry{
		value:     value,
		expiresAt: now.Add(ttl),
	}
	return true, value, nil
}
