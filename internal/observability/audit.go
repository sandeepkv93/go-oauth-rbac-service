package observability

import (
	"log/slog"
	"net/http"
)

func Audit(r *http.Request, event string, attrs ...any) {
	base := []any{
		"event", event,
		"method", r.Method,
		"path", r.URL.Path,
		"request_id", r.Header.Get("X-Request-Id"),
	}
	base = append(base, attrs...)
	slog.Info("audit", base...)
}
