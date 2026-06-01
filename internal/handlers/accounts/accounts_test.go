package accounts

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domain "github.com/nikrich/hive-v2-demo/internal/accounts"
)

// fakeStore is a minimal accounts.Store implementation backed by a map.
// Tests use it instead of NewInMemoryStore when they want to inject
// failures or pre-seed state without going through CreateAccount.
type fakeStore struct {
	createFn func(owner string) (domain.Account, error)
	getFn    func(id string) (domain.Account, error)
}

func (f *fakeStore) CreateAccount(owner string) (domain.Account, error) {
	return f.createFn(owner)
}

func (f *fakeStore) GetAccount(id string) (domain.Account, error) {
	return f.getFn(id)
}

func TestCreateAccount_HappyPath_Returns201WithAccountJSON(t *testing.T) {
	store := domain.NewInMemoryStore()

	body := strings.NewReader(`{"owner":"alice"}`)
	req := httptest.NewRequest(http.MethodPost, "/accounts", body)
	rec := httptest.NewRecorder()

	CreateAccount(store).ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusCreated; got != want {
		t.Fatalf("status code: got %d, want %d", got, want)
	}
	if got, want := resp.Header.Get("Content-Type"), "application/json"; got != want {
		t.Errorf("Content-Type: got %q, want %q", got, want)
	}

	var got domain.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Owner != "alice" {
		t.Errorf("owner: got %q, want %q", got.Owner, "alice")
	}
	if got.ID == "" {
		t.Errorf("expected a non-empty ID, got empty string")
	}
	if got.Balance != 0 {
		t.Errorf("balance: got %d, want 0", got.Balance)
	}
}

func TestCreateAccount_MissingOwner_Returns400(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "absent", body: `{}`},
		{name: "empty", body: `{"owner":""}`},
		{name: "whitespace", body: `{"owner":"   "}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := domain.NewInMemoryStore()
			req := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			CreateAccount(store).ServeHTTP(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			if got, want := resp.StatusCode, http.StatusBadRequest; got != want {
				t.Errorf("status code: got %d, want %d", got, want)
			}
		})
	}
}

func TestGetAccount_MissingID_Returns400(t *testing.T) {
	// GetAccount validates that a non-empty id PathValue is supplied.
	// (The story phrases this as the "GetAccount missing-owner 400 case";
	// GetAccount has no owner, so the equivalent required-input check
	// is on the id path value.)
	store := domain.NewInMemoryStore()

	req := httptest.NewRequest(http.MethodGet, "/accounts/", nil)
	// Intentionally do NOT set r.SetPathValue("id", ...) — simulates the
	// id being absent from the routed path.
	rec := httptest.NewRecorder()

	GetAccount(store).ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusBadRequest; got != want {
		t.Errorf("status code: got %d, want %d", got, want)
	}
}

func TestGetAccount_HappyPath_ReturnsAccountJSON(t *testing.T) {
	store := domain.NewInMemoryStore()
	created, err := store.CreateAccount("bob")
	if err != nil {
		t.Fatalf("seed CreateAccount: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/accounts/"+created.ID, nil)
	req.SetPathValue("id", created.ID)
	rec := httptest.NewRecorder()

	GetAccount(store).ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("status code: got %d, want %d", got, want)
	}
	if got, want := resp.Header.Get("Content-Type"), "application/json"; got != want {
		t.Errorf("Content-Type: got %q, want %q", got, want)
	}

	var got domain.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got != created {
		t.Errorf("account: got %+v, want %+v", got, created)
	}
}

func TestGetAccount_NotFound_Returns404(t *testing.T) {
	store := domain.NewInMemoryStore()

	req := httptest.NewRequest(http.MethodGet, "/accounts/deadbeef", nil)
	req.SetPathValue("id", "deadbeef")
	rec := httptest.NewRecorder()

	GetAccount(store).ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusNotFound; got != want {
		t.Errorf("status code: got %d, want %d", got, want)
	}
}

func TestCreateAccount_InvalidJSON_Returns400(t *testing.T) {
	store := domain.NewInMemoryStore()
	req := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()

	CreateAccount(store).ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusBadRequest; got != want {
		t.Errorf("status code: got %d, want %d", got, want)
	}
}

func TestCreateAccount_StoreError_Returns500(t *testing.T) {
	store := &fakeStore{
		createFn: func(owner string) (domain.Account, error) {
			return domain.Account{}, errors.New("boom")
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/accounts", strings.NewReader(`{"owner":"x"}`))
	rec := httptest.NewRecorder()

	CreateAccount(store).ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusInternalServerError; got != want {
		t.Errorf("status code: got %d, want %d", got, want)
	}

	// Drain body so the test goroutine has no surprise reads.
	_, _ = io.Copy(io.Discard, resp.Body)
}
