package health

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	healthcore "github.com/nikrich/hive-v2-demo/internal/health"
)

// invoke runs h against a fresh GET /health request and returns the
// captured status code, Content-Type header, and body bytes.
func invoke(t *testing.T, h http.HandlerFunc) (int, string, []byte) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return resp.StatusCode, resp.Header.Get("Content-Type"), body
}

// TestHealthAdapter_IsBehaviourallyEquivalentToHealthHandler asserts the
// adapter and the underlying internal/health.HealthHandler produce the
// same status code, Content-Type, and response body for the same request.
//
// We compare observable behaviour rather than function-pointer identity
// because Go does not define equality for function values (and the
// adapter wraps the function in http.HandlerFunc anyway, so any pointer
// comparison would be brittle and uninformative).
func TestHealthAdapter_IsBehaviourallyEquivalentToHealthHandler(t *testing.T) {
	adapterStatus, adapterCT, adapterBody := invoke(t, HealthAdapter())
	coreStatus, coreCT, coreBody := invoke(t, healthcore.HealthHandler)

	if adapterStatus != coreStatus {
		t.Errorf("status code: adapter=%d core=%d", adapterStatus, coreStatus)
	}
	if adapterCT != coreCT {
		t.Errorf("Content-Type: adapter=%q core=%q", adapterCT, coreCT)
	}
	if string(adapterBody) != string(coreBody) {
		t.Errorf("body: adapter=%q core=%q", adapterBody, coreBody)
	}
}

// TestHealthAdapter_ReturnsNonNilHandler guards against a regression where
// the adapter forgets to construct the http.HandlerFunc — a nil handler
// would surface here rather than as a confusing nil-pointer panic later.
func TestHealthAdapter_ReturnsNonNilHandler(t *testing.T) {
	if HealthAdapter() == nil {
		t.Fatal("HealthAdapter() returned nil")
	}
}
