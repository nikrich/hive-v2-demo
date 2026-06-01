package accounts

import (
	"errors"
	"sync"
	"testing"
)

func TestInMemoryStore_CreateThenGet_ReturnsSameAccount(t *testing.T) {
	store := NewInMemoryStore()

	created, err := store.CreateAccount("alice")
	if err != nil {
		t.Fatalf("CreateAccount: unexpected error: %v", err)
	}
	if created.Owner != "alice" {
		t.Errorf("Owner: got %q, want %q", created.Owner, "alice")
	}
	if created.Balance != 0 {
		t.Errorf("Balance: got %d, want 0", created.Balance)
	}
	if len(created.ID) != 8 {
		t.Errorf("ID length: got %d, want 8 (id=%q)", len(created.ID), created.ID)
	}

	fetched, err := store.GetAccount(created.ID)
	if err != nil {
		t.Fatalf("GetAccount: unexpected error: %v", err)
	}
	if fetched != created {
		t.Errorf("fetched account differs from created: got %+v, want %+v", fetched, created)
	}
}

func TestInMemoryStore_GetAccount_UnknownID_ReturnsErrAccountNotFound(t *testing.T) {
	store := NewInMemoryStore()

	_, err := store.GetAccount("deadbeef")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("error: got %v, want errors.Is(err, ErrAccountNotFound)", err)
	}
}

func TestInMemoryStore_ConcurrentCreate_ProducesDistinctIDs(t *testing.T) {
	const n = 200

	store := NewInMemoryStore()

	var wg sync.WaitGroup
	ids := make(chan string, n)
	errs := make(chan error, n)

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			acc, err := store.CreateAccount("owner")
			if err != nil {
				errs <- err
				return
			}
			ids <- acc.ID
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)

	for err := range errs {
		t.Fatalf("CreateAccount: unexpected error: %v", err)
	}

	seen := make(map[string]struct{}, n)
	for id := range ids {
		if len(id) != 8 {
			t.Errorf("ID length: got %d, want 8 (id=%q)", len(id), id)
		}
		if _, dup := seen[id]; dup {
			t.Errorf("duplicate ID generated: %q", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != n {
		t.Errorf("distinct IDs: got %d, want %d", len(seen), n)
	}
}
