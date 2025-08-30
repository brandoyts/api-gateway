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

	"github.com/stretchr/testify/assert"
)

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

			proxyHandler := NewProxyHandler(10 * time.Second)
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
	proxyHandler := NewProxyHandler(5 * time.Second)

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rr := httptest.NewRecorder()

	proxyHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "Service not found")
}

func TestServeHTTP_BackendSuccess(t *testing.T) {
	// Mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "ok")
		w.WriteHeader(http.StatusTeapot) // 418
		w.Write([]byte("backend says hi"))
	}))
	defer backend.Close()

	proxyHandler := NewProxyHandler(5 * time.Second)
	_ = proxyHandler.AddRoute("/api", backend.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/hello", nil)
	rr := httptest.NewRecorder()

	proxyHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTeapot, rr.Code)
	assert.Equal(t, "ok", rr.Header().Get("X-Test"))
	assert.Equal(t, "backend says hi", rr.Body.String())
}

func TestServeHTTP_BackendError(t *testing.T) {
	// Point to a non-routable address
	badURL := "http://127.0.0.1:65534" // port unlikely to be open

	proxyHandler := NewProxyHandler(2 * time.Second)
	_ = proxyHandler.AddRoute("/fail", badURL)

	req := httptest.NewRequest(http.MethodGet, "/fail/boom", nil)
	rr := httptest.NewRecorder()

	proxyHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadGateway, rr.Code)
	assert.Contains(t, rr.Body.String(), "Backend error")
}

func TestCreateProxyRequest_RewritesCorrectly(t *testing.T) {
	proxyHandler := NewProxyHandler(5 * time.Second)
	target, _ := url.Parse("http://backend:9000")

	body := io.NopCloser(bytes.NewBufferString("test-body"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resource", body)
	req.RemoteAddr = net.JoinHostPort("127.0.0.1", "12345")

	outReq, err := proxyHandler.createProxyRequest(req, target, "/api")
	assert.NoError(t, err)

	// URL should be rewritten (strip prefix)
	assert.Equal(t, "http://backend:9000/v1/resource", outReq.URL.String())
	assert.Equal(t, req.Method, outReq.Method)

	// Headers should be preserved + forwarded headers set
	assert.Equal(t, req.Header, outReq.Header)
	assert.Equal(t, "127.0.0.1:12345", outReq.Header.Get("X-Forwarded-For"))
	assert.Equal(t, req.Host, outReq.Header.Get("X-Forwarded-Host"))
}

func TestCreateProxyRequest_InvalidURL(t *testing.T) {
	proxyHandler := NewProxyHandler(5 * time.Second)

	// Bad request with invalid URL
	badReq := &http.Request{
		Method: http.MethodGet,
		URL:    nil, // invalid scheme
	}

	target, _ := url.Parse("http://backend:9000")
	_, err := proxyHandler.createProxyRequest(badReq, target, "/api")

	assert.Error(t, err)
}
