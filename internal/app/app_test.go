package app

import (
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/config"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/health"
)

func TestNewAssignsDependenciesAndTimeouts(t *testing.T) {
	cfg := &config.Config{
		ShutdownTimeout:              10 * time.Second,
		ShutdownHTTPDrainTimeout:     2 * time.Second,
		ShutdownObservabilityTimeout: 3 * time.Second,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &http.Server{Addr: ":8080", ReadHeaderTimeout: time.Second}
	readiness := health.NewProbeRunner(100*time.Millisecond, 50*time.Millisecond)
	stopped := false
	stop := func() { stopped = true }

	a := New(cfg, logger, server, nil, nil, nil, readiness, stop)
	if a.Config != cfg || a.Logger != logger || a.Server != server || a.Readiness != readiness {
		t.Fatal("expected app dependencies to be assigned")
	}
	if a.ShutdownTimeout != cfg.ShutdownTimeout || a.ShutdownHTTPDrainTimeout != cfg.ShutdownHTTPDrainTimeout || a.ShutdownObservabilityTimeout != cfg.ShutdownObservabilityTimeout {
		t.Fatal("expected app shutdown timeouts copied from config")
	}

	a.StopBackgroundTasks()
	if !stopped {
		t.Fatal("expected stop callback to be set")
	}
}
