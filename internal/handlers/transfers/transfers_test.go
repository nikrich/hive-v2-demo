package transfers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nikrich/hive-v2-demo/internal/accounts"
	"github.com/nikrich/hive-v2-demo/internal/idempotency"
	"github.com/nikrich/hive-v2-demo/internal/ledger"
)

// newRequest builds an httptest request whose path value {id} is wired up the
// same way Go 1.22's net/http router would do it for routes like
// "POST /accounts/{id}/transfers". Tests don't run through a real mux, so we
// set the path value explicitly.
func newRequest(t *testing.T, fromID, body, idemKey string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/accounts/"+fromID+"/transfers", strings.NewReader(body))
	req.SetPathValue("id", fromID)
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	return req
}

// seedAccounts creates two accounts in a fresh store and returns the store
// plus the two account IDs.
func seedAccounts(t *testing.T) (accounts.Store, string, string) {
	t.Helper()
	store := accounts.NewInMemoryStore()
	from, err := store.CreateAccount("alice")
	if err != nil {
		t.Fatalf("seed from: %v", err)
	}
	to, err := store.CreateAccount("bob")
	if err != nil {
		t.Fatalf("seed to: %v", err)
	}
	return store, from.ID, to.ID
}

func TestTransfer_HappyPath_201_WritesBalancedLedgerEntries(t *testing.T) {
	store, fromID, toID := seedAccounts(t)
	journal := ledger.New()
	keys := idempotency.NewInMemoryKeyStore()

	body := `{"to_account":"` + toID + `","amount":750}`
	req := newRequest(t, fromID, body, "")
	rec := httptest.NewRecorder()

	Transfer(store, journal, keys)(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusCreated; got != want {
		t.Fatalf("status: got %d, want %d", got, want)
	}

	var decoded transferResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if decoded.From != fromID {
		t.Errorf("from: got %q, want %q", decoded.From, fromID)
	}
	if decoded.To != toID {
		t.Errorf("to: got %q, want %q", decoded.To, toID)
	}
	if decoded.Amount != 750 {
		t.Errorf("amount: got %d, want 750", decoded.Amount)
	}
	if len(decoded.TransferID) != 8 {
		t.Errorf("transfer_id length: got %d, want 8 (id=%q)", len(decoded.TransferID), decoded.TransferID)
	}

	fromEntries := journal.List(fromID)
	if len(fromEntries) != 1 || fromEntries[0].Amount != -750 {
		t.Errorf("from-account ledger: got %+v, want one entry of -750", fromEntries)
	}
	toEntries := journal.List(toID)
	if len(toEntries) != 1 || toEntries[0].Amount != 750 {
		t.Errorf("to-account ledger: got %+v, want one entry of +750", toEntries)
	}
}

func TestTransfer_IdempotencyReplay_DoesNotDoubleDebit(t *testing.T) {
	store, fromID, toID := seedAccounts(t)
	journal := ledger.New()
	keys := idempotency.NewInMemoryKeyStore()
	handler := Transfer(store, journal, keys)

	body := `{"to_account":"` + toID + `","amount":250}`

	// First call: should succeed with 201.
	rec1 := httptest.NewRecorder()
	handler(rec1, newRequest(t, fromID, body, "key-abc"))
	if got, want := rec1.Result().StatusCode, http.StatusCreated; got != want {
		t.Fatalf("first call status: got %d, want %d", got, want)
	}

	// Second call with the same Idempotency-Key: should be replayed.
	rec2 := httptest.NewRecorder()
	handler(rec2, newRequest(t, fromID, body, "key-abc"))

	resp2 := rec2.Result()
	defer resp2.Body.Close()
	if got, want := resp2.StatusCode, http.StatusOK; got != want {
		t.Fatalf("replay status: got %d, want %d", got, want)
	}
	bodyBytes, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("read replay body: %v", err)
	}
	if got, want := string(bodyBytes), "replayed"; got != want {
		t.Errorf("replay body: got %q, want %q", got, want)
	}

	// Ledger must still hold exactly one debit and one credit.
	fromEntries := journal.List(fromID)
	if len(fromEntries) != 1 {
		t.Errorf("from-account entries after replay: got %d, want 1 (entries=%+v)", len(fromEntries), fromEntries)
	}
	toEntries := journal.List(toID)
	if len(toEntries) != 1 {
		t.Errorf("to-account entries after replay: got %d, want 1 (entries=%+v)", len(toEntries), toEntries)
	}
}

func TestTransfer_MissingToAccount_Returns400(t *testing.T) {
	store, fromID, _ := seedAccounts(t)
	journal := ledger.New()
	keys := idempotency.NewInMemoryKeyStore()

	// to_account omitted entirely.
	body := `{"amount":100}`
	rec := httptest.NewRecorder()
	Transfer(store, journal, keys)(rec, newRequest(t, fromID, body, ""))

	if got, want := rec.Result().StatusCode, http.StatusBadRequest; got != want {
		t.Errorf("status: got %d, want %d", got, want)
	}
	if entries := journal.List(fromID); len(entries) != 0 {
		t.Errorf("ledger must be empty on validation failure, got %+v", entries)
	}
}

func TestTransfer_EmptyToAccount_Returns400(t *testing.T) {
	store, fromID, _ := seedAccounts(t)
	journal := ledger.New()
	keys := idempotency.NewInMemoryKeyStore()

	body := `{"to_account":"","amount":100}`
	rec := httptest.NewRecorder()
	Transfer(store, journal, keys)(rec, newRequest(t, fromID, body, ""))

	if got, want := rec.Result().StatusCode, http.StatusBadRequest; got != want {
		t.Errorf("status: got %d, want %d", got, want)
	}
}

func TestTransfer_UnknownFromAccount_Returns404(t *testing.T) {
	store, _, toID := seedAccounts(t)
	journal := ledger.New()
	keys := idempotency.NewInMemoryKeyStore()

	body := `{"to_account":"` + toID + `","amount":100}`
	rec := httptest.NewRecorder()
	Transfer(store, journal, keys)(rec, newRequest(t, "ffffffff", body, ""))

	if got, want := rec.Result().StatusCode, http.StatusNotFound; got != want {
		t.Errorf("status: got %d, want %d", got, want)
	}
}

func TestTransfer_UnknownToAccount_Returns404(t *testing.T) {
	store, fromID, _ := seedAccounts(t)
	journal := ledger.New()
	keys := idempotency.NewInMemoryKeyStore()

	body := `{"to_account":"ffffffff","amount":100}`
	rec := httptest.NewRecorder()
	Transfer(store, journal, keys)(rec, newRequest(t, fromID, body, ""))

	if got, want := rec.Result().StatusCode, http.StatusNotFound; got != want {
		t.Errorf("status: got %d, want %d", got, want)
	}
}

func TestTransfer_ZeroAmount_Returns400(t *testing.T) {
	store, fromID, toID := seedAccounts(t)
	journal := ledger.New()
	keys := idempotency.NewInMemoryKeyStore()

	body := `{"to_account":"` + toID + `","amount":0}`
	rec := httptest.NewRecorder()
	Transfer(store, journal, keys)(rec, newRequest(t, fromID, body, ""))

	if got, want := rec.Result().StatusCode, http.StatusBadRequest; got != want {
		t.Errorf("status: got %d, want %d", got, want)
	}
}

func TestTransfer_NegativeAmount_Returns400(t *testing.T) {
	store, fromID, toID := seedAccounts(t)
	journal := ledger.New()
	keys := idempotency.NewInMemoryKeyStore()

	body := `{"to_account":"` + toID + `","amount":-50}`
	rec := httptest.NewRecorder()
	Transfer(store, journal, keys)(rec, newRequest(t, fromID, body, ""))

	if got, want := rec.Result().StatusCode, http.StatusBadRequest; got != want {
		t.Errorf("status: got %d, want %d", got, want)
	}
}
