package response

import (
	"encoding/json"
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *apiError   `json:"error,omitempty"`
	Meta    meta        `json:"meta"`
}

type apiError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type meta struct {
	RequestID string    `json:"request_id"`
	Timestamp time.Time `json:"timestamp"`
}

func JSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Success: true, Data: data, Meta: buildMeta(r)})
}

func Error(w http.ResponseWriter, r *http.Request, status int, code, message string, details interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Success: false, Error: &apiError{Code: code, Message: message, Details: details}, Meta: buildMeta(r)})
}

func buildMeta(r *http.Request) meta {
	id := chimiddleware.GetReqID(r.Context())
	if id == "" {
		id = r.Header.Get("X-Request-Id")
	}
	if id == "" {
		id = "req-unknown"
	}
	return meta{RequestID: id, Timestamp: time.Now().UTC()}
}
