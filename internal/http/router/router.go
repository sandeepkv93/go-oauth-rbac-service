package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/health"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/http/handler"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/http/middleware"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/http/response"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/security"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/service"
)

type Dependencies struct {
	AuthHandler                *handler.AuthHandler
	UserHandler                *handler.UserHandler
	AdminHandler               *handler.AdminHandler
	JWTManager                 *security.JWTManager
	RBACService                service.RBACAuthorizer
	PermissionResolver         service.PermissionResolver
	CORSOrigins                []string
	AuthRateLimitRPM           int
	PasswordForgotRateLimitRPM int
	APIRateLimitRPM            int
	GlobalRateLimiter          GlobalRateLimiterFunc
	AuthRateLimiter            AuthRateLimiterFunc
	ForgotRateLimiter          ForgotRateLimiterFunc
	Idempotency                IdempotencyMiddlewareFactory
	Readiness                  *health.ProbeRunner
	EnableOTelHTTP             bool
}

type GlobalRateLimiterFunc func(http.Handler) http.Handler
type AuthRateLimiterFunc func(http.Handler) http.Handler
type ForgotRateLimiterFunc func(http.Handler) http.Handler
type IdempotencyMiddlewareFactory func(scope string) func(http.Handler) http.Handler

func NewRouter(dep Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.StructuredRequestLogger)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CORS(dep.CORSOrigins))
	r.Use(middleware.BodyLimit(1 << 20))
	if dep.GlobalRateLimiter != nil {
		r.Use(dep.GlobalRateLimiter)
	} else {
		r.Use(middleware.NewRateLimiter(dep.APIRateLimitRPM, time.Minute).Middleware())
	}

	authLimiter := dep.AuthRateLimiter
	if authLimiter == nil {
		authLimiter = middleware.NewRateLimiter(dep.AuthRateLimitRPM, time.Minute).Middleware()
	}
	forgotLimiter := dep.ForgotRateLimiter
	if forgotLimiter == nil {
		forgotLimiter = middleware.NewRateLimiter(dep.PasswordForgotRateLimitRPM, time.Minute).Middleware()
	}

	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if dep.Readiness == nil {
			response.JSON(w, r, http.StatusOK, map[string]any{"status": "ready", "checks": []any{}})
			return
		}
		ready, results := dep.Readiness.Ready(r.Context())
		if ready {
			response.JSON(w, r, http.StatusOK, map[string]any{"status": "ready", "checks": results})
			return
		}
		response.Error(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNREADY", "dependencies are not ready", map[string]any{"checks": results})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.With(authLimiter).Get("/google/login", dep.AuthHandler.GoogleLogin)
			r.With(authLimiter).Get("/google/callback", dep.AuthHandler.GoogleCallback)
			registerChain := []func(http.Handler) http.Handler{authLimiter}
			if dep.Idempotency != nil {
				registerChain = append(registerChain, dep.Idempotency("auth.local.register"))
			}
			r.With(registerChain...).Post("/local/register", dep.AuthHandler.LocalRegister)
			r.With(authLimiter).Post("/local/login", dep.AuthHandler.LocalLogin)
			r.With(authLimiter).Post("/local/verify/request", dep.AuthHandler.LocalVerifyRequest)
			r.With(authLimiter).Post("/local/verify/confirm", dep.AuthHandler.LocalVerifyConfirm)
			forgotChain := []func(http.Handler) http.Handler{forgotLimiter}
			if dep.Idempotency != nil {
				forgotChain = append(forgotChain, dep.Idempotency("auth.local.password.forgot"))
			}
			r.With(forgotChain...).Post("/local/password/forgot", dep.AuthHandler.LocalPasswordForgot)
			r.With(authLimiter).Post("/local/password/reset", dep.AuthHandler.LocalPasswordReset)
			r.Group(func(r chi.Router) {
				r.Use(middleware.CSRFMiddleware)
				r.With(authLimiter).Post("/refresh", dep.AuthHandler.Refresh)
				r.With(middleware.AuthMiddleware(dep.JWTManager)).Post("/logout", dep.AuthHandler.Logout)
				r.With(middleware.AuthMiddleware(dep.JWTManager), authLimiter).Post("/local/change-password", dep.AuthHandler.LocalChangePassword)
			})
		})

		r.With(middleware.AuthMiddleware(dep.JWTManager)).Get("/me", dep.UserHandler.Me)
		r.With(middleware.AuthMiddleware(dep.JWTManager)).Get("/me/sessions", dep.UserHandler.Sessions)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(dep.JWTManager))
			r.Use(middleware.CSRFMiddleware)
			r.Delete("/me/sessions/{session_id}", dep.UserHandler.RevokeSession)
			r.Post("/me/sessions/revoke-others", dep.UserHandler.RevokeOtherSessions)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(dep.JWTManager))
			r.With(middleware.RequirePermission(dep.RBACService, dep.PermissionResolver, "users:read")).Get("/users", dep.AdminHandler.ListUsers)
			userRoleChain := []func(http.Handler) http.Handler{middleware.RequirePermission(dep.RBACService, dep.PermissionResolver, "users:write")}
			if dep.Idempotency != nil {
				userRoleChain = append(userRoleChain, dep.Idempotency("admin.users.roles.patch"))
			}
			r.With(userRoleChain...).Patch("/users/{id}/roles", dep.AdminHandler.SetUserRoles)
			r.With(middleware.RequirePermission(dep.RBACService, dep.PermissionResolver, "roles:read")).Get("/roles", dep.AdminHandler.ListRoles)
			roleCreateChain := []func(http.Handler) http.Handler{middleware.RequirePermission(dep.RBACService, dep.PermissionResolver, "roles:write")}
			if dep.Idempotency != nil {
				roleCreateChain = append(roleCreateChain, dep.Idempotency("admin.roles.create"))
			}
			r.With(roleCreateChain...).Post("/roles", dep.AdminHandler.CreateRole)
			r.With(middleware.RequirePermission(dep.RBACService, dep.PermissionResolver, "roles:write")).Patch("/roles/{id}", dep.AdminHandler.UpdateRole)
			r.With(middleware.RequirePermission(dep.RBACService, dep.PermissionResolver, "roles:write")).Delete("/roles/{id}", dep.AdminHandler.DeleteRole)
			r.With(middleware.RequirePermission(dep.RBACService, dep.PermissionResolver, "permissions:read")).Get("/permissions", dep.AdminHandler.ListPermissions)
			r.With(middleware.RequirePermission(dep.RBACService, dep.PermissionResolver, "permissions:write")).Post("/permissions", dep.AdminHandler.CreatePermission)
			r.With(middleware.RequirePermission(dep.RBACService, dep.PermissionResolver, "permissions:write")).Patch("/permissions/{id}", dep.AdminHandler.UpdatePermission)
			r.With(middleware.RequirePermission(dep.RBACService, dep.PermissionResolver, "permissions:write")).Delete("/permissions/{id}", dep.AdminHandler.DeletePermission)
			r.With(middleware.RequirePermission(dep.RBACService, dep.PermissionResolver, "roles:write")).Post("/rbac/sync", dep.AdminHandler.SyncRBAC)
		})
	})

	var h http.Handler = r
	if dep.EnableOTelHTTP {
		h = otelhttp.NewHandler(r, "http.server")
	}
	return h
}
