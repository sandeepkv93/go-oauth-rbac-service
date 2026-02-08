package observability

import (
	"fmt"
	"log/slog"
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
)

func Audit(r *http.Request, event string, attrs ...any) {
	msg := "audit"
	sc := trace.SpanContextFromContext(r.Context())
	if sc.IsValid() {
		msg = fmt.Sprintf("audit trace_id=%s span_id=%s", sc.TraceID().String(), sc.SpanID().String())
	}
	base := []any{
		"event", event,
		"method", r.Method,
		"path", r.URL.Path,
		"request_id", requestID(r),
	}
	base = append(base, attrs...)
	slog.InfoContext(r.Context(), msg, base...)
}

func requestID(r *http.Request) string {
	if id := chimiddleware.GetReqID(r.Context()); id != "" {
		return id
	}
	return r.Header.Get("X-Request-Id")
}
