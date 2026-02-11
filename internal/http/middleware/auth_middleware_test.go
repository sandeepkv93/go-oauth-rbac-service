package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/security"
)

func TestAuthMiddlewareMissingTokenReturnsUnauthorized(t *testing.T) {
	jwtMgr := security.NewJWTManager(
		"iss",
		"aud",
		"abcdefghijklmnopqrstuvwxyz123456",
		"abcdefghijklmnopqrstuvwxyz654321",
	)
	h := AuthMiddleware(jwtMgr)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing token, got %d", rr.Code)
	}
}

func TestAuthMiddlewareValidBearerTokenPasses(t *testing.T) {
	jwtMgr := security.NewJWTManager(
		"iss",
		"aud",
		"abcdefghijklmnopqrstuvwxyz123456",
		"abcdefghijklmnopqrstuvwxyz654321",
	)
	token, err := jwtMgr.SignAccessToken(42, nil, nil, 15*time.Minute)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	h := AuthMiddleware(jwtMgr)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for valid token, got %d", rr.Code)
	}
}
