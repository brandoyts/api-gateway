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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

	ctx, span := p.Telemetry.TraceStart(r.Context(), "api_gateway_request")
	defer span.End()

	// Add request attributes to the span
	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.url", r.URL.String()),
		attribute.String("http.path", r.URL.Path),
	)

	var targetUrl *url.URL
	var longestPrefix string

	for prefix, backendUrl := range p.Routes {
		if len(prefix) > len(longestPrefix) && strings.HasPrefix(r.URL.Path, prefix) {
			longestPrefix = prefix
			targetUrl = &backendUrl
		}
	}

	if targetUrl == nil {
		p.Telemetry.LogErrorln(ErrServiceNotFound, r.URL.Path)
		span.RecordError(ErrServiceNotFound)
		span.SetStatus(codes.Error, ErrServiceNotFound.Error())
		span.SetAttributes(
			attribute.String("http.response.status_code", string(rune(http.StatusNotFound))),
		)
		http.Error(w, ErrServiceNotFound.Error(), http.StatusNotFound)
		return
	}

	proxyRequest, err := p.createProxyRequest(r, targetUrl, longestPrefix)
	if err != nil {
		p.Telemetry.LogErrorln(ErrCreateProxyRequest, err)
		span.RecordError(ErrCreateProxyRequest)
		span.SetStatus(codes.Error, ErrCreateProxyRequest.Error())
		span.SetAttributes(
			attribute.String("http.response.status_code", string(rune(http.StatusInternalServerError))),
		)
		http.Error(w, ErrCreateProxyRequest.Error(), http.StatusInternalServerError)
		return
	}

	//  inject trace context into outbound request headers
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(proxyRequest.Header))

	// send request to backend
	proxyResponse, err := p.Client.Do(proxyRequest)
	if err != nil {
		p.Telemetry.LogErrorln(ErrBackendResponse, proxyRequest.URL.String(), err)
		span.RecordError(ErrBackendResponse)
		span.SetStatus(codes.Error, ErrBackendResponse.Error())
		span.SetAttributes(
			attribute.String("http.response.status_code", string(rune(http.StatusBadGateway))),
		)
		http.Error(w, ErrBackendResponse.Error(), http.StatusBadGateway)
		return
	}
	defer proxyResponse.Body.Close()

	// copy headers from backend response
	for key, values := range proxyResponse.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	if proxyResponse.StatusCode >= http.StatusBadRequest {
		p.Telemetry.LogErrorln(ErrRouteNotExist, r.URL.Path)
		span.RecordError(ErrRouteNotExist)
		span.SetStatus(codes.Error, ErrRouteNotExist.Error())
		span.SetAttributes(
			attribute.String("http.response.status_code", string(rune(http.StatusNotFound))),
		)
		http.Error(w, ErrRouteNotExist.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(proxyResponse.StatusCode)
	io.Copy(w, proxyResponse.Body)

	p.Telemetry.LogInfof(
		"Proxy request: %v %v -> %v, status: %v, latency: %v",
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
