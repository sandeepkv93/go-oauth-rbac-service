package app

import (
	"log/slog"
	"net/http"

	"go-oauth-rbac-service/internal/config"
	"go-oauth-rbac-service/internal/observability"
)

type App struct {
	Config        *config.Config
	Logger        *slog.Logger
	Server        *http.Server
	Observability *observability.Runtime
}

func New(cfg *config.Config, logger *slog.Logger, server *http.Server, runtime *observability.Runtime) *App {
	return &App{Config: cfg, Logger: logger, Server: server, Observability: runtime}
}
