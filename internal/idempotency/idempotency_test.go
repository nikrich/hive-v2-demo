package idempotency

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTrySet_FreshKey_StoresAndReturnsOurValue(t *testing.T) {
	s := NewInMemoryKeyStore()

	won, stored, err := s.TrySet("order-42", "resp-A", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !won {
		t.Fatalf("expected won=true for fresh key, got false")
	}
	if stored != "resp-A" {
		t.Fatalf("expected stored=%q, got %q", "resp-A", stored)
	}
}

func TestTrySet_DuplicateWithinTTL_ReturnsOriginalValue(t *testing.T) {
	s := NewInMemoryKeyStore()

	if won, _, err := s.TrySet("order-42", "resp-A", time.Minute); err != nil || !won {
		t.Fatalf("first TrySet should win: won=%v err=%v", won, err)
	}

	won, stored, err := s.TrySet("order-42", "resp-B", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if won {
		t.Fatalf("expected won=false for duplicate within TTL, got true")
	}
	if stored != "resp-A" {
		t.Fatalf("expected original value %q to be returned, got %q", "resp-A", stored)
	}
}

func TestTrySet_AfterTTL_TreatsKeyAsAbsent(t *testing.T) {
	// Inject a fake clock so we can advance time deterministically without
	// sleeping — keeps the test fast and free of flakiness.
	var nowNanos atomic.Int64
	nowNanos.Store(time.Unix(0, 0).UnixNano())
	clock := func() time.Time { return time.Unix(0, nowNanos.Load()) }

	s := NewInMemoryKeyStoreWithClock(clock)

	if won, _, err := s.TrySet("order-42", "resp-A", 10*time.Millisecond); err != nil || !won {
		t.Fatalf("first TrySet should win: won=%v err=%v", won, err)
	}

	// Advance the clock past the TTL.
	nowNanos.Add(int64(50 * time.Millisecond))

	won, stored, err := s.TrySet("order-42", "resp-B", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !won {
		t.Fatalf("expected won=true after TTL expired, got false")
	}
	if stored != "resp-B" {
		t.Fatalf("expected new value %q after expiry, got %q", "resp-B", stored)
	}
}

func TestTrySet_AfterTTL_RealClock(t *testing.T) {
	// Belt-and-braces: also exercise the real wall clock with a tiny TTL, so we
	// know the default constructor's clock wiring is correct.
	s := NewInMemoryKeyStore()

	if won, _, err := s.TrySet("k", "v1", 5*time.Millisecond); err != nil || !won {
		t.Fatalf("first TrySet should win: won=%v err=%v", won, err)
	}
	time.Sleep(20 * time.Millisecond)

	won, stored, err := s.TrySet("k", "v2", 5*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !won || stored != "v2" {
		t.Fatalf("expected (true, %q, nil) after wall-clock expiry, got (%v, %q)", "v2", won, stored)
	}
}

func TestTrySet_ConcurrentWritersExactlyOneWins(t *testing.T) {
	// Race-test the mutex: many goroutines hammer the same key; exactly one
	// must observe won=true, and every other caller must see that winner's
	// value.
	s := NewInMemoryKeyStore()

	const n = 64
	var wg sync.WaitGroup
	wg.Add(n)

	type result struct {
		won    bool
		stored string
	}
	results := make([]result, n)

	start := make(chan struct{})
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			won, stored, _ := s.TrySet("hot-key", values[i%len(values)], time.Minute)
			results[i] = result{won: won, stored: stored}
		}()
	}
	close(start)
	wg.Wait()

	winners := 0
	var winningValue string
	for _, r := range results {
		if r.won {
			winners++
			winningValue = r.stored
		}
	}
	if winners != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", winners)
	}
	for i, r := range results {
		if r.stored != winningValue {
			t.Fatalf("result[%d] stored=%q, expected all readers to see winner=%q", i, r.stored, winningValue)
		}
	}
}

var values = []string{"a", "b", "c", "d", "e", "f", "g", "h"}
