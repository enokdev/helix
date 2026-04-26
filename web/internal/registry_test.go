package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouteRegistry_RegisterGeneratedRoutes(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*RouteRegistry)
		verify  func(*testing.T, *RouteRegistry)
		wantErr bool
	}{
		{
			name: "register valid routes",
			setup: func(r *RouteRegistry) {
				routes := []RouteInfo{
					{Method: "GET", Path: "/users", Handler: func() {}, Controller: "UserController"},
				}
				err := r.RegisterGeneratedRoutes("UserController", routes...)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, r *RouteRegistry) {
				routes := r.GetGeneratedRoutes("UserController")
				assert.Equal(t, 1, len(routes))
				assert.Equal(t, "GET", routes[0].Method)
				assert.Equal(t, "/users", routes[0].Path)
			},
		},
		{
			name: "multiple routes for same controller",
			setup: func(r *RouteRegistry) {
				routes := []RouteInfo{
					{Method: "GET", Path: "/users", Handler: func() {}, Controller: "UserController"},
					{Method: "POST", Path: "/users", Handler: func() {}, Controller: "UserController"},
					{Method: "DELETE", Path: "/users/:id", Handler: func() {}, Controller: "UserController"},
				}
				err := r.RegisterGeneratedRoutes("UserController", routes...)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, r *RouteRegistry) {
				routes := r.GetGeneratedRoutes("UserController")
				assert.Equal(t, 3, len(routes))
			},
		},
		{
			name: "empty controller name error",
			setup: func(r *RouteRegistry) {
				err := r.RegisterGeneratedRoutes("", RouteInfo{Method: "GET", Path: "/", Handler: func() {}})
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "empty controller name")
			},
		},
		{
			name: "empty route method error",
			setup: func(r *RouteRegistry) {
				err := r.RegisterGeneratedRoutes("UserController", RouteInfo{Method: "", Path: "/users", Handler: func() {}})
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "empty method")
			},
		},
		{
			name: "empty route path error",
			setup: func(r *RouteRegistry) {
				err := r.RegisterGeneratedRoutes("UserController", RouteInfo{Method: "GET", Path: "", Handler: func() {}})
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "empty path")
			},
		},
		{
			name: "nil handler error",
			setup: func(r *RouteRegistry) {
				err := r.RegisterGeneratedRoutes("UserController", RouteInfo{Method: "GET", Path: "/users", Handler: nil})
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "nil handler")
			},
		},
		{
			name: "has generated routes",
			setup: func(r *RouteRegistry) {
				assert.False(t, r.HasGeneratedRoutes("UserController"))
				routes := []RouteInfo{
					{Method: "GET", Path: "/users", Handler: func() {}, Controller: "UserController"},
				}
				_ = r.RegisterGeneratedRoutes("UserController", routes...)
				assert.True(t, r.HasGeneratedRoutes("UserController"))
			},
		},
		{
			name: "empty routes list is safe",
			setup: func(r *RouteRegistry) {
				err := r.RegisterGeneratedRoutes("UserController")
				require.NoError(t, err)
			},
			verify: func(t *testing.T, r *RouteRegistry) {
				assert.False(t, r.HasGeneratedRoutes("UserController"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &RouteRegistry{routes: make(map[string][]RouteInfo)}
			tt.setup(registry)
			if tt.verify != nil {
				tt.verify(t, registry)
			}
		})
	}
}

func TestErrorHandlerRegistry_RegisterGeneratedErrorHandlers(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*ErrorHandlerRegistry)
		verify  func(*testing.T, *ErrorHandlerRegistry)
		wantErr bool
	}{
		{
			name: "register valid handlers",
			setup: func(r *ErrorHandlerRegistry) {
				handlers := []ErrorHandlerInfo{
					{ErrorType: "UserNotFoundError", Handler: func() {}},
				}
				err := r.RegisterGeneratedErrorHandlers(handlers...)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, r *ErrorHandlerRegistry) {
				handler, ok := r.GetGeneratedErrorHandler("UserNotFoundError")
				assert.True(t, ok)
				assert.NotNil(t, handler.Handler)
			},
		},
		{
			name: "multiple error handlers",
			setup: func(r *ErrorHandlerRegistry) {
				handlers := []ErrorHandlerInfo{
					{ErrorType: "NotFoundError", Handler: func() {}},
					{ErrorType: "ValidationError", Handler: func() {}},
				}
				err := r.RegisterGeneratedErrorHandlers(handlers...)
				require.NoError(t, err)
			},
			verify: func(t *testing.T, r *ErrorHandlerRegistry) {
				_, ok1 := r.GetGeneratedErrorHandler("NotFoundError")
				_, ok2 := r.GetGeneratedErrorHandler("ValidationError")
				assert.True(t, ok1)
				assert.True(t, ok2)
			},
		},
		{
			name: "empty error type error",
			setup: func(r *ErrorHandlerRegistry) {
				err := r.RegisterGeneratedErrorHandlers(ErrorHandlerInfo{ErrorType: "", Handler: func() {}})
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "empty error type")
			},
		},
		{
			name: "nil handler error",
			setup: func(r *ErrorHandlerRegistry) {
				err := r.RegisterGeneratedErrorHandlers(ErrorHandlerInfo{ErrorType: "NotFoundError", Handler: nil})
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "nil handler")
			},
		},
		{
			name: "has generated error handlers",
			setup: func(r *ErrorHandlerRegistry) {
				assert.False(t, r.HasGeneratedErrorHandlers())
				handlers := []ErrorHandlerInfo{
					{ErrorType: "NotFoundError", Handler: func() {}},
				}
				_ = r.RegisterGeneratedErrorHandlers(handlers...)
				assert.True(t, r.HasGeneratedErrorHandlers())
			},
		},
		{
			name: "empty handlers list is safe",
			setup: func(r *ErrorHandlerRegistry) {
				err := r.RegisterGeneratedErrorHandlers()
				require.NoError(t, err)
			},
			verify: func(t *testing.T, r *ErrorHandlerRegistry) {
				assert.False(t, r.HasGeneratedErrorHandlers())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &ErrorHandlerRegistry{handlers: make(map[string]ErrorHandlerInfo)}
			tt.setup(registry)
			if tt.verify != nil {
				tt.verify(t, registry)
			}
		})
	}
}

func TestGlobalRegistries(t *testing.T) {
	t.Run("global route registry is accessible", func(t *testing.T) {
		reg := GlobalRouteRegistry()
		assert.NotNil(t, reg)
	})

	t.Run("global error handler registry is accessible", func(t *testing.T) {
		reg := GlobalErrorHandlerRegistry()
		assert.NotNil(t, reg)
	})
}
