package observability

import (
	"context"
	"errors"
	"log/slog"

	"go-oauth-rbac-service/internal/config"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Runtime struct {
	MeterProvider  *sdkmetric.MeterProvider
	TracerProvider *sdktrace.TracerProvider
}

func InitRuntime(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*Runtime, error) {
	mp, err := InitMetrics(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	tp, err := InitTracing(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	return &Runtime{MeterProvider: mp, TracerProvider: tp}, nil
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	var errs []error
	if r.MeterProvider != nil {
		if err := r.MeterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if r.TracerProvider != nil {
		if err := r.TracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
