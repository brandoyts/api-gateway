package main

import (
	"context"
	"net/http"
	"os"

	"github.com/brandoyts/api-gateway/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

var telem *telemetry.Telemetry

func main() {
	ctx := context.Background()

	// Load telemetry config
	telemetryConfiguration, err := telemetry.NewTelemetryConfiguration("./config/userTelemetryConfig.yml")
	if err != nil {
		panic("error reading telemetry configuration file: " + err.Error())
	}

	// Init telemetry
	telem, err = telemetry.NewTelemetry(ctx, *telemetryConfiguration)
	if err != nil {
		panic("failed to create user service telemetry: " + err.Error())
	}
	defer telem.Shutdown(ctx)

	// Register handlers
	mux := http.NewServeMux()
	mux.Handle("/profile", http.HandlerFunc(profileHandler))

	// Wrap the mux with telemetry tracing middleware
	wrappedMux := telemetryMiddleware(telem, telemetryConfiguration.ServiceName, mux)

	addr := ":6001"
	telem.LogInfo("🚀 user service started on port", addr)
	if err := http.ListenAndServe(addr, wrappedMux); err != nil {
		telem.LogErrorln("server failed:", err)
		os.Exit(1)
	}
}

// Handlers
func profileHandler(w http.ResponseWriter, r *http.Request) {
	telem.LogInfo("handling /profile request")
	w.Write([]byte("user service: profile\n"))
}

// telemetryMiddleware extracts trace context and starts a new span for each request
func telemetryMiddleware(telem *telemetry.Telemetry, serviceName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract parent context from incoming headers
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// Start span
		ctx, span := telem.TraceStart(ctx, "user_service")
		defer span.End()

		// Add useful attributes to the span
		span.SetAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.Path),
			attribute.String("http.client_ip", r.RemoteAddr),
		)

		// Pass context downstream
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
