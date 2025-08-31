package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/brandoyts/api-gateway/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type ProxyHandler struct {
	Telemetry telemetry.TelemetryProvider
	Routes    map[string]url.URL
	Client    *http.Client
}

func NewProxyHandler(telemetryProvider telemetry.TelemetryProvider, requestTimeout time.Duration) *ProxyHandler {
	return &ProxyHandler{
		Telemetry: telemetryProvider,
		Routes:    make(map[string]url.URL),
		Client: &http.Client{
			Timeout: requestTimeout,
		},
	}
}

func (p *ProxyHandler) AddRoute(prefix string, backendUrl string) error {
	target, err := url.ParseRequestURI(backendUrl)
	if err != nil {
		return err
	}

	p.Routes[prefix] = *target

	return nil
}

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

	ctx, span := p.Telemetry.TraceStart(ctx, "api gateway request")
	defer span.End()

	var targetUrl *url.URL
	var longestPrefix string

	for prefix, backendUrl := range p.Routes {
		if len(prefix) > len(longestPrefix) && strings.HasPrefix(r.URL.Path, prefix) {
			longestPrefix = prefix
			targetUrl = &backendUrl
		}
	}

	if targetUrl == nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		p.Telemetry.LogInfo("No route found for %s", r.URL.Path)
		return
	}

	proxyRequest, err := p.createProxyRequest(r, targetUrl, longestPrefix)
	if err != nil {
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		p.Telemetry.LogInfo("Error creating proxy request: %v", err)
		return
	}

	//  inject trace context into outbound request headers
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(proxyRequest.Header))

	// send request to backend
	proxyResponse, err := p.Client.Do(proxyRequest)
	if err != nil {
		http.Error(w, "Backend error", http.StatusBadGateway)
		p.Telemetry.LogInfo("Backend error for %s: %v", proxyRequest.URL.String(), err)
		return
	}
	defer proxyResponse.Body.Close()

	// copy headers from backend response
	for key, values := range proxyResponse.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(proxyResponse.StatusCode)
	io.Copy(w, proxyResponse.Body)

	p.Telemetry.LogInfo(
		"Proxy request: %s %s -> %s, status: %d, latency: %v",
		r.Method,
		r.URL.Path,
		targetUrl.String(),
		proxyResponse.StatusCode,
		time.Since(startTime),
	)
}

func (p *ProxyHandler) createProxyRequest(r *http.Request, target *url.URL, prefix string) (*http.Request, error) {
	if r.URL == nil {
		return nil, fmt.Errorf("request URL is nil")
	}

	outUrl := *r.URL
	outUrl.Scheme = target.Scheme
	outUrl.Host = target.Host
	outUrl.Path = strings.TrimPrefix(outUrl.Path, prefix)

	outRequest, err := http.NewRequestWithContext(r.Context(), r.Method, outUrl.String(), r.Body)
	if err != nil {
		return nil, err
	}

	outRequest.Header = r.Header

	outRequest.Header.Set("X-Forwarded-For", r.RemoteAddr)
	outRequest.Header.Set("X-Forwarded-Host", r.Host)
	outRequest.Header.Set("X-Forwarded-Proto", r.URL.Scheme)
	outRequest.Header.Set("X-Forwarded-Proto", target.Scheme)

	return outRequest, nil
}
