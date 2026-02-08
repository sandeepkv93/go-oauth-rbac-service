package middleware

import (
	"net/http"

	"go-oauth-rbac-service/internal/http/response"
	"go-oauth-rbac-service/internal/service"
)

func RequirePermission(rbac service.RBACAuthorizer, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				response.Error(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "missing auth context", nil)
				return
			}
			if !rbac.HasPermission(claims.Permissions, permission) {
				response.Error(w, r, http.StatusForbidden, "FORBIDDEN", "insufficient permission", map[string]string{"required": permission})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
