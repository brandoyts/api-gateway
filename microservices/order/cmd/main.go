package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/brandoyts/api-gateway/internal/telemetry"
	"go.opentelemetry.io/otel/propagation"
)

func main() {
	ctx := context.Background()

	mux := http.NewServeMux()

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

	mux.HandleFunc("/create", func(w http.ResponseWriter, r *http.Request) {
		// extract traces context from headers
		ctx := telem.Propagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// start span with extracted context
		_, span := telem.TraceStart(ctx, "orderCreateHandler")
		defer span.End()

		telem.LogInfo(w, "order service: create")
	})

	mux.HandleFunc("/cancel", func(w http.ResponseWriter, r *http.Request) {
		// extract traces context from headers
		ctx := telem.Propagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// start span with extracted context
		_, span := telem.TraceStart(ctx, "orderCancelHandler")
		defer span.End()

		telem.LogInfo(w, "order service: cancel")
	})

	addr := ":6002"
	log.Printf("🚀 order service started on port %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
