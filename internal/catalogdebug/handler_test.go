package catalogdebug

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPHandler_unauthorizedWithoutTokenWhenAnonDisabled(t *testing.T) {
	h := HTTPHandler("secret", false)
	req := httptest.NewRequest(http.MethodGet, "/debug/capability-catalog", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestHTTPHandler_okWithBearer(t *testing.T) {
	h := HTTPHandler("secret", false)
	req := httptest.NewRequest(http.MethodGet, "/debug/capability-catalog", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type=%q", ct)
	}
	if !strings.Contains(rec.Body.String(), `"coreEmployees"`) {
		t.Fatalf("expected catalog JSON")
	}
}

func TestPublicHTTPHandler_okWithoutAuth(t *testing.T) {
	h := PublicHTTPHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/public/capability-catalog", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type=%q", ct)
	}
	if !strings.Contains(rec.Body.String(), `"skills"`) {
		t.Fatalf("expected catalog JSON")
	}
}
