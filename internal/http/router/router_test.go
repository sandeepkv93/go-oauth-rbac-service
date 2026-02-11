package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/health"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/security"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/service"
)

type unhealthyChecker struct{}

func (unhealthyChecker) Check(ctx context.Context) health.CheckResult {
	return health.CheckResult{Name: "db", Healthy: false, Error: "db down"}
}

func newRouterTestDeps() Dependencies {
	return Dependencies{
		AuthHandler:                nil,
		UserHandler:                nil,
		AdminHandler:               nil,
		JWTManager:                 security.NewJWTManager("iss", "aud", "abcdefghijklmnopqrstuvwxyz123456", "abcdefghijklmnopqrstuvwxyz654321"),
		RBACService:                service.NewRBACService(),
		PermissionResolver:         nil,
		CORSOrigins:                []string{"http://localhost"},
		AuthRateLimitRPM:           1000,
		PasswordForgotRateLimitRPM: 1000,
		APIRateLimitRPM:            1000,
		RouteRateLimitPolicies:     nil,
		Idempotency:                nil,
		EnableOTelHTTP:             false,
	}
}

func perform(r http.Handler, method, target string, headers map[string]string, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.RemoteAddr = "10.10.10.10:1234"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func bearerToken(t *testing.T, jwtMgr *security.JWTManager, perms []string) string {
	t.Helper()
	token, err := jwtMgr.SignAccessToken(42, []string{"admin"}, perms, time.Hour)
	if err != nil {
		t.Fatalf("sign access token: %v", err)
	}
	return token
}

func TestRouterHealthReadyNilAndUnreadyBranches(t *testing.T) {
	t.Run("nil readiness returns ready", func(t *testing.T) {
		dep := newRouterTestDeps()
		dep.Readiness = nil
		r := NewRouter(dep)

		rr := perform(r, http.MethodGet, "/health/ready", nil, nil, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), `"status":"ready"`) {
			t.Fatalf("expected ready status payload, got %s", rr.Body.String())
		}
	})

	t.Run("unready dependency returns 503", func(t *testing.T) {
		dep := newRouterTestDeps()
		dep.Readiness = health.NewProbeRunner(time.Second, 0, unhealthyChecker{})
		r := NewRouter(dep)

		rr := perform(r, http.MethodGet, "/health/ready", nil, nil, "")
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), `"code":"DEPENDENCY_UNREADY"`) {
			t.Fatalf("expected DEPENDENCY_UNREADY error envelope, got %s", rr.Body.String())
		}
	})
}

func TestRouterHealthLiveAlwaysOKWithDefaultLimiter(t *testing.T) {
	dep := newRouterTestDeps()
	r := NewRouter(dep)

	rr := perform(r, http.MethodGet, "/health/live", nil, nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"status":"ok"`) {
		t.Fatalf("expected health live payload, got %s", rr.Body.String())
	}
}

func TestRouterFallbackGlobalRateLimiterWhenCustomNil(t *testing.T) {
	dep := newRouterTestDeps()
	dep.APIRateLimitRPM = 1
	dep.GlobalRateLimiter = nil
	r := NewRouter(dep)

	first := perform(r, http.MethodGet, "/health/live", nil, nil, "")
	if first.Code != http.StatusOK {
		t.Fatalf("first request expected 200, got %d", first.Code)
	}
	second := perform(r, http.MethodGet, "/health/live", nil, nil, "")
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request expected 429 from fallback limiter, got %d", second.Code)
	}
}

func TestRouterRoutePolicyOverridesPerNamedPolicy(t *testing.T) {
	dep := newRouterTestDeps()
	policyHits := map[string]int{}
	policy := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				policyHits[name]++
				w.Header().Set("X-Policy", name)
				w.WriteHeader(http.StatusNoContent)
			})
		}
	}
	dep.RouteRateLimitPolicies = RouteRateLimitPolicies{
		RoutePolicyLogin:      policy(RoutePolicyLogin),
		RoutePolicyRefresh:    policy(RoutePolicyRefresh),
		RoutePolicyAdminWrite: policy(RoutePolicyAdminWrite),
		RoutePolicyAdminSync:  policy(RoutePolicyAdminSync),
	}
	r := NewRouter(dep)
	token := bearerToken(t, dep.JWTManager, []string{"users:write", "roles:write", "permissions:write"})

	rr := perform(r, http.MethodPost, "/api/v1/auth/local/login", nil, nil, `{"email":"u@example.com","password":"x"}`)
	if rr.Code != http.StatusNoContent || rr.Header().Get("X-Policy") != RoutePolicyLogin {
		t.Fatalf("login policy override not applied, status=%d policy=%q", rr.Code, rr.Header().Get("X-Policy"))
	}

	rr = perform(r, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"X-CSRF-Token": "csrf"}, []*http.Cookie{{Name: "csrf_token", Value: "csrf"}, {Name: "refresh_token", Value: "r"}}, "")
	if rr.Code != http.StatusNoContent || rr.Header().Get("X-Policy") != RoutePolicyRefresh {
		t.Fatalf("refresh policy override not applied, status=%d policy=%q", rr.Code, rr.Header().Get("X-Policy"))
	}

	rr = perform(r, http.MethodPatch, "/api/v1/admin/users/11/roles", map[string]string{"Authorization": "Bearer " + token}, nil, `{"role_ids":[1]}`)
	if rr.Code != http.StatusNoContent || rr.Header().Get("X-Policy") != RoutePolicyAdminWrite {
		t.Fatalf("admin_write policy override not applied, status=%d policy=%q body=%s", rr.Code, rr.Header().Get("X-Policy"), rr.Body.String())
	}

	rr = perform(r, http.MethodPost, "/api/v1/admin/rbac/sync", map[string]string{"Authorization": "Bearer " + token}, nil, "")
	if rr.Code != http.StatusNoContent || rr.Header().Get("X-Policy") != RoutePolicyAdminSync {
		t.Fatalf("admin_sync policy override not applied, status=%d policy=%q body=%s", rr.Code, rr.Header().Get("X-Policy"), rr.Body.String())
	}

	if policyHits[RoutePolicyLogin] != 1 || policyHits[RoutePolicyRefresh] != 1 || policyHits[RoutePolicyAdminWrite] != 1 || policyHits[RoutePolicyAdminSync] != 1 {
		t.Fatalf("expected one hit per policy override, got %+v", policyHits)
	}
}

func TestRouterCSRFScopeOnSensitiveRoutes(t *testing.T) {
	dep := newRouterTestDeps()
	r := NewRouter(dep)
	token := bearerToken(t, dep.JWTManager, []string{"users:write"})

	cases := []struct {
		name    string
		method  string
		path    string
		headers map[string]string
		cookies []*http.Cookie
		body    string
	}{
		{
			name:   "refresh",
			method: http.MethodPost,
			path:   "/api/v1/auth/refresh",
			cookies: []*http.Cookie{
				{Name: "refresh_token", Value: "rt"},
			},
		},
		{
			name:    "logout",
			method:  http.MethodPost,
			path:    "/api/v1/auth/logout",
			headers: map[string]string{"Authorization": "Bearer " + token},
		},
		{
			name:    "change-password",
			method:  http.MethodPost,
			path:    "/api/v1/auth/local/change-password",
			headers: map[string]string{"Authorization": "Bearer " + token},
			body:    `{"current_password":"a","new_password":"b"}`,
		},
		{
			name:    "revoke-session",
			method:  http.MethodDelete,
			path:    "/api/v1/me/sessions/12",
			headers: map[string]string{"Authorization": "Bearer " + token},
		},
		{
			name:    "revoke-others",
			method:  http.MethodPost,
			path:    "/api/v1/me/sessions/revoke-others",
			headers: map[string]string{"Authorization": "Bearer " + token},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := perform(r, tc.method, tc.path, tc.headers, tc.cookies, tc.body)
			if rr.Code != http.StatusForbidden {
				t.Fatalf("expected 403 csrf rejection, got %d body=%s", rr.Code, rr.Body.String())
			}
			var env map[string]any
			_ = json.NewDecoder(rr.Body).Decode(&env)
			errObj, _ := env["error"].(map[string]any)
			if code, _ := errObj["code"].(string); code != "FORBIDDEN" {
				t.Fatalf("expected FORBIDDEN error code, got %+v", errObj)
			}
		})
	}
}
