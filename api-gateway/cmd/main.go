package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brandoyts/api-gateway/api-gateway/config"
	"github.com/brandoyts/api-gateway/api-gateway/internal/proxy"
	"github.com/brandoyts/api-gateway/internal/telemetry"
)

// helper to chain middlewares
type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, m ...Middleware) http.Handler {
	for i := len(m) - 1; i >= 0; i-- {
		h = m[i](h)
	}
	return h
}

func main() {
	ctx := context.Background()

	// load config
	gatewayConfiguration, errGatewayConfig := config.NewGatewayConfiguration()
	if errGatewayConfig != nil {
		log.Fatalf("error on reading gateway configuration file: %v", errGatewayConfig)
		os.Exit(1)
	}

	telemetryConfiguration, errTelemetryConfig := telemetry.NewTelemetryConfiguration("./config/gatewayTelemetryConfig.yml")
	if errTelemetryConfig != nil {
		log.Fatalf("error on reading telemetry configuration file: %v", errTelemetryConfig)
		os.Exit(1)
	}

	// init telemetry
	telem, err := telemetry.NewTelemetry(ctx, *telemetryConfiguration)
	if err != nil {
		log.Fatalf("failed to create telemetry: %v", err)
		os.Exit(1)
	}
	defer telem.Shutdown(ctx)

	// init proxy handler
	proxyHandler := proxy.NewProxyHandler(telem, gatewayConfiguration.RequestTimeout)

	for _, route := range gatewayConfiguration.Routes {
		if err := proxyHandler.AddRoute(route.Prefix, route.BackendUrl); err != nil {
			log.Fatalf("error adding routes: %v\n", err)
		}
		log.Printf("%v route added successfully\n", route.Prefix)
	}

	// wrap proxy handler with telemetry middlewares
	handler := Chain(proxyHandler,
		telem.LogRequest,
		telem.MeterRequestDuration,
		telem.MeterRequestsInFlight,
	)

	// server setup
	gateway := &http.Server{
		Addr:    gatewayConfiguration.ListenAddress,
		Handler: handler,
	}

	// start server in goroutine
	go func() {
		log.Printf("Gateway listening on %s\n", gatewayConfiguration.ListenAddress)
		if err := gateway.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v\n", err)
		}
	}()

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctxShutdown, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	log.Println("Shutting down server...")
	if err := gateway.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}
	log.Println("Server exited gracefully")
}
