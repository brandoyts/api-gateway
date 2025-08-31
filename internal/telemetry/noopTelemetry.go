package telemetry

import (
	"context"
	"net/http"
	"os"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// NoopTelemetry is a no-op implementation of the TelemetryProvider interface.
type NoopTelemetry struct {
	serviceName string
	prop        propagation.TextMapPropagator
}

// NewNoopTelemetry creates a new NoopTelemetry instance.
func NewNoopTelemetry(cfg TelemetryConfiguration) (*NoopTelemetry, error) {
	return &NoopTelemetry{
		serviceName: cfg.ServiceName,
		// 🔹 no-op propagator (doesn't modify headers)
		prop: propagation.NewCompositeTextMapPropagator(),
	}, nil
}

// GetServiceName returns the service name.
func (t *NoopTelemetry) GetServiceName() string { return t.serviceName }

// LogInfo logs nothing.
func (t *NoopTelemetry) LogInfo(args ...interface{}) {}

// LogErrorln logs nothing.
func (t *NoopTelemetry) LogErrorln(args ...interface{}) {}

// LogFatalln logs nothing, then exits.
func (t *NoopTelemetry) LogFatalln(args ...interface{}) {
	os.Exit(1)
}

// LogRequest is a no-op middleware for net/http.
func (t *NoopTelemetry) LogRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// MeterRequestDuration is a no-op middleware for net/http.
func (t *NoopTelemetry) MeterRequestDuration(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// MeterRequestsInFlight is a no-op middleware for net/http.
func (t *NoopTelemetry) MeterRequestsInFlight(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// TraceStart returns the context and span unchanged.
func (t *NoopTelemetry) TraceStart(ctx context.Context, name string) (context.Context, trace.Span) {
	return ctx, trace.SpanFromContext(ctx)
}

// MeterInt64Histogram returns nil.
func (t *NoopTelemetry) MeterInt64Histogram(metric Metric) (metric.Int64Histogram, error) {
	return nil, nil
}

// MeterInt64UpDownCounter returns nil.
func (t *NoopTelemetry) MeterInt64UpDownCounter(metric Metric) (metric.Int64UpDownCounter, error) {
	return nil, nil
}

// Propagator returns a no-op propagator.
func (t *NoopTelemetry) Propagator() propagation.TextMapPropagator {
	return t.prop
}

// Shutdown does nothing.
func (t *NoopTelemetry) Shutdown(ctx context.Context) {}
