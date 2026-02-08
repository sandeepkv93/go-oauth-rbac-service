package observability

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"go-oauth-rbac-service/internal/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

type AppMetrics struct {
	authLoginCounter   metric.Int64Counter
	authRefreshCounter metric.Int64Counter
	authLogoutCounter  metric.Int64Counter
	adminRoleCounter   metric.Int64Counter
}

var (
	metricsMu  sync.RWMutex
	appMetrics *AppMetrics
)

func InitMetrics(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*sdkmetric.MeterProvider, error) {
	if !cfg.OTELMetricsEnabled {
		mp := sdkmetric.NewMeterProvider()
		otel.SetMeterProvider(mp)
		logger.Info("otel metrics disabled")
		return mp, nil
	}

	opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.OTELExporterOTLPEndpoint)}
	if cfg.OTELExporterOTLPInsecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create otlp metric exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.OTELServiceName),
			attribute.String("deployment.environment", cfg.OTELEnvironment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create metric resource: %w", err)
	}

	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(cfg.OTELMetricsExportInterval))
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)
	otel.SetMeterProvider(mp)

	meter := mp.Meter("go-oauth-rbac-service")
	loginCounter, err := meter.Int64Counter("auth.login.attempts")
	if err != nil {
		return nil, err
	}
	refreshCounter, err := meter.Int64Counter("auth.refresh.attempts")
	if err != nil {
		return nil, err
	}
	logoutCounter, err := meter.Int64Counter("auth.logout.attempts")
	if err != nil {
		return nil, err
	}
	adminRoleCounter, err := meter.Int64Counter("admin.role.mutations")
	if err != nil {
		return nil, err
	}

	metricsMu.Lock()
	appMetrics = &AppMetrics{
		authLoginCounter:   loginCounter,
		authRefreshCounter: refreshCounter,
		authLogoutCounter:  logoutCounter,
		adminRoleCounter:   adminRoleCounter,
	}
	metricsMu.Unlock()

	logger.Info("otel metrics initialized", "endpoint", cfg.OTELExporterOTLPEndpoint)
	return mp, nil
}

func RecordAuthLogin(provider, status string) {
	metricsMu.RLock()
	m := appMetrics
	metricsMu.RUnlock()
	if m == nil {
		return
	}
	m.authLoginCounter.Add(context.Background(), 1,
		metric.WithAttributes(
			attribute.String("provider", provider),
			attribute.String("status", status),
		),
	)
}

func RecordAuthRefresh(status string) {
	metricsMu.RLock()
	m := appMetrics
	metricsMu.RUnlock()
	if m == nil {
		return
	}
	m.authRefreshCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("status", status)))
}

func RecordAuthLogout(status string) {
	metricsMu.RLock()
	m := appMetrics
	metricsMu.RUnlock()
	if m == nil {
		return
	}
	m.authLogoutCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("status", status)))
}

func RecordAdminRoleMutation(action string) {
	metricsMu.RLock()
	m := appMetrics
	metricsMu.RUnlock()
	if m == nil {
		return
	}
	m.adminRoleCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("action", action)))
}
