package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/security"
)

type testRBACAuthorizer struct {
	allow bool
}

func (a testRBACAuthorizer) HasPermission(_ []string, _ string) bool {
	return a.allow
}

type testPermissionResolver struct {
	perms []string
	err   error
}

func (r testPermissionResolver) ResolvePermissions(_ context.Context, _ *security.Claims) ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.perms, nil
}

func (r testPermissionResolver) InvalidateUser(_ context.Context, _ uint) error { return nil }
func (r testPermissionResolver) InvalidateAll(_ context.Context) error          { return nil }

func TestRequirePermissionDenied(t *testing.T) {
	mw := RequirePermission(testRBACAuthorizer{allow: false}, nil, "admin:read")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ClaimsContextKey, &security.Claims{Permissions: []string{"user:read"}}))
	rr := httptest.NewRecorder()

	mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("expected middleware to block request")
	})).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestRequirePermissionResolverError(t *testing.T) {
	resolverErr := errors.New("resolver unavailable")
	mw := RequirePermission(testRBACAuthorizer{allow: true}, testPermissionResolver{err: resolverErr}, "admin:read")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ClaimsContextKey, &security.Claims{Permissions: []string{"admin:read"}}))
	rr := httptest.NewRecorder()

	mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("expected middleware to block request")
	})).ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestRequirePermissionAllowed(t *testing.T) {
	mw := RequirePermission(testRBACAuthorizer{allow: true}, testPermissionResolver{perms: []string{"admin:read"}}, "admin:read")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ClaimsContextKey, &security.Claims{Permissions: []string{"admin:read"}}))
	rr := httptest.NewRecorder()

	called := false
	mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !called {
		t.Fatal("expected wrapped handler to be called")
	}
}
