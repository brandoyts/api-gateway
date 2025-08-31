package proxy

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/brandoyts/api-gateway/internal/telemetry"
	"github.com/stretchr/testify/assert"
)

func newTestProxyHandler(t *testing.T, timeout time.Duration) *ProxyHandler {
	t.Helper()

	// use noop telemetry for tests
	telem, err := telemetry.NewNoopTelemetry(telemetry.TelemetryConfiguration{})
	if err != nil {
		t.Fatalf("failed to create noop telemetry: %v", err)
	}

	return NewProxyHandler(telem, timeout)
}

func TestAddRoute(t *testing.T) {
	tests := []struct {
		name         string
		prefix       string
		url          string
		expectToFail bool
	}{
		{"valid route with scheme", "/user", "http://localhost:8000", false},
		{"valid route without scheme (should pass)", "/order", "localhost:8000", false},
		{"empty url should fail", "/product", "", true},
		{"invalid url without port should fail", "/product", "localhost", true},
		{"valid https route", "/secure", "https://example.com", false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			proxyHandler := newTestProxyHandler(t, 10*time.Second)
			err := proxyHandler.AddRoute(tc.prefix, tc.url)

			if tc.expectToFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				storedURL, exists := proxyHandler.Routes[tc.prefix]
				assert.True(t, exists)
				assert.Equal(t, tc.url, storedURL.String())
			}
		})
	}
}

func TestServeHTTP_NoRouteFound(t *testing.T) {
	proxyHandler := newTestProxyHandler(t, 5*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rr := httptest.NewRecorder()

	proxyHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "Service not found")
}

func TestServeHTTP_BackendSuccess(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "ok")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("backend says hi"))
	}))
	defer backend.Close()

	proxyHandler := newTestProxyHandler(t, 5*time.Second)
	_ = proxyHandler.AddRoute("/api", backend.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/hello", nil)
	rr := httptest.NewRecorder()

	proxyHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTeapot, rr.Code)
	assert.Equal(t, "ok", rr.Header().Get("X-Test"))
	assert.Equal(t, "backend says hi", rr.Body.String())
}

func TestServeHTTP_BackendError(t *testing.T) {
	// non-routable port
	badURL := "http://127.0.0.1:65534"

	proxyHandler := newTestProxyHandler(t, 2*time.Second)
	_ = proxyHandler.AddRoute("/fail", badURL)

	req := httptest.NewRequest(http.MethodGet, "/fail/boom", nil)
	rr := httptest.NewRecorder()

	proxyHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadGateway, rr.Code)
	assert.Contains(t, rr.Body.String(), "Backend error")
}

func TestCreateProxyRequest_RewritesCorrectly(t *testing.T) {
	proxyHandler := newTestProxyHandler(t, 5*time.Second)
	target, _ := url.Parse("http://backend:9000")

	body := io.NopCloser(bytes.NewBufferString("test-body"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resource", body)
	req.RemoteAddr = net.JoinHostPort("127.0.0.1", "12345")

	outReq, err := proxyHandler.createProxyRequest(req, target, "/api")
	assert.NoError(t, err)

	assert.Equal(t, "http://backend:9000/v1/resource", outReq.URL.String())
	assert.Equal(t, req.Method, outReq.Method)

	assert.Equal(t, req.Header, outReq.Header)
	assert.Equal(t, "127.0.0.1:12345", outReq.Header.Get("X-Forwarded-For"))
	assert.Equal(t, req.Host, outReq.Header.Get("X-Forwarded-Host"))
}

func TestCreateProxyRequest_InvalidURL(t *testing.T) {
	proxyHandler := newTestProxyHandler(t, 5*time.Second)

	badReq := &http.Request{
		Method: http.MethodGet,
		URL:    nil,
	}

	target, _ := url.Parse("http://backend:9000")
	_, err := proxyHandler.createProxyRequest(badReq, target, "/api")

	assert.Error(t, err)
}
