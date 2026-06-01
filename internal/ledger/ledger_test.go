package ledger

import (
	"sync"
	"testing"
)

func TestAppendThenListReturnsEntriesInOrder(t *testing.T) {
	l := New()

	e1, err := l.Append("acct-1", 100)
	if err != nil {
		t.Fatalf("Append #1 returned error: %v", err)
	}
	e2, err := l.Append("acct-1", -25)
	if err != nil {
		t.Fatalf("Append #2 returned error: %v", err)
	}
	e3, err := l.Append("acct-1", 7)
	if err != nil {
		t.Fatalf("Append #3 returned error: %v", err)
	}

	got := l.List("acct-1")
	if len(got) != 3 {
		t.Fatalf("List length = %d, want 3", len(got))
	}

	want := []Entry{e1, e2, e3}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("entry[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestAppendIsolatesAccounts(t *testing.T) {
	l := New()

	if _, err := l.Append("a", 1); err != nil {
		t.Fatalf("Append a: %v", err)
	}
	if _, err := l.Append("b", 2); err != nil {
		t.Fatalf("Append b: %v", err)
	}
	if _, err := l.Append("a", 3); err != nil {
		t.Fatalf("Append a again: %v", err)
	}

	if got := l.List("a"); len(got) != 2 {
		t.Errorf("List(a) length = %d, want 2", len(got))
	}
	if got := l.List("b"); len(got) != 1 {
		t.Errorf("List(b) length = %d, want 1", len(got))
	}
}

func TestListUnknownAccountReturnsEmptySlice(t *testing.T) {
	l := New()

	got := l.List("does-not-exist")
	if got == nil {
		t.Fatalf("List of unknown account returned nil; want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("List of unknown account returned len=%d, want 0", len(got))
	}
}

func TestListReturnsCopyNotInternalSlice(t *testing.T) {
	l := New()
	if _, err := l.Append("acct", 10); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got := l.List("acct")
	got[0] = Entry{ID: "tampered"}

	again := l.List("acct")
	if again[0].ID == "tampered" {
		t.Errorf("mutating returned slice mutated internal storage")
	}
}

func TestConcurrentAppendsAllPersistWithUniqueIDs(t *testing.T) {
	l := New()

	const goroutines = 16
	const perGoroutine = 64
	const total = goroutines * perGoroutine

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if _, err := l.Append("shared", 1); err != nil {
					t.Errorf("Append returned error: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	got := l.List("shared")
	if len(got) != total {
		t.Fatalf("List length = %d, want %d", len(got), total)
	}

	seen := make(map[string]struct{}, total)
	for _, e := range got {
		if _, dup := seen[e.ID]; dup {
			t.Fatalf("duplicate ID %q in ledger", e.ID)
		}
		seen[e.ID] = struct{}{}
	}
}
