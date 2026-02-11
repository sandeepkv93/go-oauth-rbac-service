package config

import (
	"context"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	configMetricsOnce sync.Once
	configCounter     metric.Int64Counter
)

func recordConfigValidationEvent(ctx context.Context, profile, outcome, errorClass string) {
	configMetricsOnce.Do(func() {
		counter, err := otel.Meter("secure-observable-go-backend-starter-kit").Int64Counter("config.validation.events")
		if err == nil {
			configCounter = counter
		}
	})
	if configCounter == nil {
		return
	}
	configCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("profile", normalizeConfigProfile(profile)),
		attribute.String("outcome", outcome),
		attribute.String("error_class", errorClass),
	))
}

func normalizeConfigProfile(profile string) string {
	v := strings.TrimSpace(strings.ToLower(profile))
	if v == "" {
		return "unknown"
	}
	return v
}

func classifyConfigLoadError(err error) string {
	if err == nil {
		return "none"
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "validate config:"):
		return "validation"
	case strings.Contains(msg, "parse "):
		return "parse"
	default:
		return "load"
	}
}
