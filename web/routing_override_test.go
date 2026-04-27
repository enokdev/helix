package web_test

import (
	"context"
	"net/http"
	"testing"

	helix "github.com/enokdev/helix"
	"github.com/enokdev/helix/web"
	"github.com/stretchr/testify/assert"
)

// TestControllerRouteOverride validates that struct tag helix:"route:..." overrides convention-based route
func TestControllerRouteOverride(t *testing.T) {
	tests := []struct {
		name        string
		controller  any
		expectedErr bool
	}{
		{
			name:        "valid override",
			controller:  &OverrideController{},
			expectedErr: false,
		},
		{
			name:        "invalid override empty route",
			controller:  &EmptyRouteController{},
			expectedErr: true,
		},
		{
			name:        "invalid override malformed route",
			controller:  &InvalidRouteController{},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := web.RegisterController(newMockHTTPServer(), tt.controller)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// OverrideController has a helix:"route:..." tag that overrides the default convention
type OverrideController struct {
	helix.Controller `helix:"route:/v1/users"`
}

func (c *OverrideController) Index() {}

// EmptyRouteController has an empty route tag
type EmptyRouteController struct {
	helix.Controller `helix:"route:"`
}

func (c *EmptyRouteController) Index() {}

// InvalidRouteController has an invalid route tag
type InvalidRouteController struct {
	helix.Controller `helix:"route:invalid-no-slash"`
}

func (c *InvalidRouteController) Index() {}

// newMockHTTPServer creates a mock HTTPServer for testing
func newMockHTTPServer() web.HTTPServer {
	return &mockHTTPServer{
		routes: make(map[string][]routeHandler),
	}
}

type mockHTTPServer struct {
	routes map[string][]routeHandler
}

type routeHandler struct {
	method  string
	path    string
	handler web.HandlerFunc
}

func (m *mockHTTPServer) Start(_ string) error {
	return nil
}

func (m *mockHTTPServer) Stop(_ context.Context) error {
	return nil
}

func (m *mockHTTPServer) RegisterRoute(method, path string, handler web.HandlerFunc) error {
	m.routes[method] = append(m.routes[method], routeHandler{method, path, handler})
	return nil
}

func (m *mockHTTPServer) IsGeneratedOnly() bool {
	return false
}

func (m *mockHTTPServer) ServeHTTP(_ *http.Request) (*http.Response, error) {
	// Stub for testing
	return nil, nil
}
