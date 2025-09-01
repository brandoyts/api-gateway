package proxy

import "errors"

var (
	ErrServiceNotFound    = errors.New("service not found")
	ErrCreateProxyRequest = errors.New("failed to create proxy request")
	ErrBackendResponse    = errors.New("something went wrong in backend service")
	ErrRouteNotExist      = errors.New("route not exists")
)
