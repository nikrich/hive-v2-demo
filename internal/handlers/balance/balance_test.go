package balance

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nikrich/hive-v2-demo/internal/accounts"
	"github.com/nikrich/hive-v2-demo/internal/ledger"
)

// newRequestWithID builds an *http.Request whose PathValue("id") returns
// id. Using *http.ServeMux here keeps the test close to how the handler
// is actually invoked at runtime and avoids hand-poking SetPathValue.
func newRequestWithID(t *testing.T, h http.Handler, id string) *httptest.ResponseRecorder {
	t.Helper()

	mux := http.NewServeMux()
	mux.Handle("GET /accounts/{id}/balance", h)

	req := httptest.NewRequest(http.MethodGet, "/accounts/"+id+"/balance", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestBalance_SumsPositiveAndNegativeEntries(t *testing.T) {
	store := accounts.NewInMemoryStore()
	journal := ledger.New()

	acc, err := store.CreateAccount("Ada Lovelace")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Seed a mix of credits and debits; expected balance = 1000 - 250 + 75 - 25 = 800.
	for _, amount := range []int64{1000, -250, 75, -25} {
		if _, err := journal.Append(acc.ID, amount); err != nil {
			t.Fatalf("append ledger entry %d: %v", amount, err)
		}
	}

	// Also seed an entry for an unrelated account to make sure the handler
	// does not accidentally include other accounts in the sum.
	other, err := store.CreateAccount("Grace Hopper")
	if err != nil {
		t.Fatalf("create other account: %v", err)
	}
	if _, err := journal.Append(other.ID, 9999); err != nil {
		t.Fatalf("append unrelated entry: %v", err)
	}

	rec := newRequestWithID(t, Balance(store, journal), acc.ID)
	resp := rec.Result()
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status code: got %d, want %d", got, want)
	}
	if got, want := resp.Header.Get("Content-Type"), "application/json"; got != want {
		t.Errorf("Content-Type: got %q, want %q", got, want)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var got response
	if err := json.Unmarshal(bodyBytes, &got); err != nil {
		t.Fatalf("decode body %q: %v", string(bodyBytes), err)
	}

	if got.AccountID != acc.ID {
		t.Errorf("account_id: got %q, want %q", got.AccountID, acc.ID)
	}
	if got.Balance != 800 {
		t.Errorf("balance: got %d, want %d", got.Balance, int64(800))
	}
}

func TestBalance_ZeroWhenNoEntries(t *testing.T) {
	store := accounts.NewInMemoryStore()
	journal := ledger.New()

	acc, err := store.CreateAccount("Alan Turing")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	rec := newRequestWithID(t, Balance(store, journal), acc.ID)
	resp := rec.Result()
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status code: got %d, want %d", got, want)
	}

	var got response
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.AccountID != acc.ID {
		t.Errorf("account_id: got %q, want %q", got.AccountID, acc.ID)
	}
	if got.Balance != 0 {
		t.Errorf("balance: got %d, want 0", got.Balance)
	}
}

func TestBalance_404WhenAccountUnknown(t *testing.T) {
	store := accounts.NewInMemoryStore()
	journal := ledger.New()

	rec := newRequestWithID(t, Balance(store, journal), "deadbeef")
	resp := rec.Result()
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusNotFound; got != want {
		t.Fatalf("status code: got %d, want %d", got, want)
	}
}
