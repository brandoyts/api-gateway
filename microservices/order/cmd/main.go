package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/brandoyts/api-gateway/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func main() {
	ctx := context.Background()

	telemetryConfiguration, errTelemetryConfig := telemetry.NewTelemetryConfiguration("./config/orderTelemetryConfig.yml")
	if errTelemetryConfig != nil {
		log.Fatalf("error on reading telemetry configuration file: %v", errTelemetryConfig)
		os.Exit(1)
	}

	// init telemetry
	telem, err := telemetry.NewTelemetry(ctx, *telemetryConfiguration)
	if err != nil {
		fmt.Println("failed to create order service telemetry:", err)
		os.Exit(1)
	}
	defer telem.Shutdown(ctx)

	mux := http.NewServeMux()

	// Wrap all handlers with telemetry tracing middleware
	mux.Handle("/create", telemetryMiddleware(telem, "orderCreateHandler", http.HandlerFunc(orderCreateHandler)))
	mux.Handle("/cancel", telemetryMiddleware(telem, "orderCancelHandler", http.HandlerFunc(orderCancelHandler)))

	addr := ":6002"
	log.Printf("🚀 order service started on port %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// Handlers
func orderCreateHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "order service: create")
}

func orderCancelHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "order service: cancel")
}

// telemetryMiddleware extracts trace context and starts a new span
func telemetryMiddleware(telem *telemetry.Telemetry, spanName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract parent context from incoming headers
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// Start span with parent context
		ctx, span := telem.TraceStart(ctx, spanName)
		defer span.End()

		// Pass down context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
