package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFMiddlewareRejectsMissingCookie(t *testing.T) {
	h := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/local/login", nil)
	req.Header.Set("X-CSRF-Token", "token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without csrf cookie, got %d", rr.Code)
	}
}

func TestCSRFMiddlewareRejectsMismatch(t *testing.T) {
	h := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/local/login", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "cookie-value"})
	req.Header.Set("X-CSRF-Token", "header-value")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for csrf mismatch, got %d", rr.Code)
	}
}

func TestCSRFMiddlewareAllowsMatchingToken(t *testing.T) {
	h := CSRFMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/local/login", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "match"})
	req.Header.Set("X-CSRF-Token", "match")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for valid csrf token, got %d", rr.Code)
	}
}

func TestCSRFPathGroup(t *testing.T) {
	cases := map[string]string{
		"/":                            "root",
		"/api/v1/auth/local/login":     "api/auth",
		"/api/v1/admin/roles":          "api/admin",
		"/api/v1/me/sessions":          "api/me",
		"/health/ready":                "health",
		"/api/v1/password/reset/token": "api/password",
	}
	for input, expected := range cases {
		if got := csrfPathGroup(input); got != expected {
			t.Fatalf("csrfPathGroup(%q)=%q want %q", input, got, expected)
		}
	}
}
