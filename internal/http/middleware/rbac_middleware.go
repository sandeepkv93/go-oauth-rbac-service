package middleware

import (
	"net/http"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/http/response"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/observability"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/service"
)

func RequirePermission(rbac service.RBACAuthorizer, resolver service.PermissionResolver, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				response.Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth context", nil)
				return
			}
			perms := claims.Permissions
			if resolver != nil {
				resolved, err := resolver.ResolvePermissions(r.Context(), claims)
				if err != nil {
					observability.RecordRBACAuthorizationEvent(r.Context(), permission, "resolver_error")
					response.Error(w, r, http.StatusServiceUnavailable, "RBAC_UNAVAILABLE", "permission resolution unavailable", nil)
					return
				}
				perms = resolved
			}
			if !rbac.HasPermission(perms, permission) {
				observability.RecordRBACAuthorizationEvent(r.Context(), permission, "denied")
				response.Error(w, r, http.StatusForbidden, "FORBIDDEN", "insufficient permission", map[string]string{"required": permission})
				return
			}
			observability.RecordRBACAuthorizationEvent(r.Context(), permission, "allowed")
			next.ServeHTTP(w, r)
		})
	}
}
