package internal

import (
	"fmt"
	"log/slog"
	"sync"
)

// RouteInfo contains metadata about a generated route registration.
type RouteInfo struct {
	Method      string // HTTP method (GET, POST, etc.)
	Path        string // URL path
	Handler     interface{}
	Controller  string // Name of the controller type
	HandlerName string // Name of the handler method
}

// ErrorHandlerInfo contains metadata about a generated error handler registration.
type ErrorHandlerInfo struct {
	ErrorType  string      // Name of the error type
	Handler    interface{} // Handler function
	MethodName string      // Name of the handler method
}

// RouteRegistry stores generated route registrations.
type RouteRegistry struct {
	mu     sync.RWMutex
	routes map[string][]RouteInfo // controllerName → routes
}

// ErrorHandlerRegistry stores generated error handler registrations.
type ErrorHandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[string]ErrorHandlerInfo // errorType → handler
}

var (
	// globalRouteRegistry holds all generated routes.
	globalRouteRegistry = &RouteRegistry{
		routes: make(map[string][]RouteInfo),
	}

	// globalErrorHandlerRegistry holds all generated error handlers.
	globalErrorHandlerRegistry = &ErrorHandlerRegistry{
		handlers: make(map[string]ErrorHandlerInfo),
	}
)

// RegisterGeneratedRoutes registers routes for a given controller.
// This is called by generated code during app initialization.
func (r *RouteRegistry) RegisterGeneratedRoutes(controller string, routes ...RouteInfo) error {
	if controller == "" {
		return fmt.Errorf("web/internal: register generated routes: empty controller name")
	}
	if len(routes) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, route := range routes {
		if route.Method == "" {
			return fmt.Errorf("web/internal: register generated route for %s: empty method", controller)
		}
		if route.Path == "" {
			return fmt.Errorf("web/internal: register generated route for %s %s: empty path", controller, route.Method)
		}
		if route.Handler == nil {
			return fmt.Errorf("web/internal: register generated route for %s %s %s: nil handler", controller, route.Method, route.Path)
		}
	}

	r.routes[controller] = routes
	slog.Debug("registered generated routes", "controller", controller, "count", len(routes))
	return nil
}

// GetGeneratedRoutes retrieves routes for a controller from the registry.
func (r *RouteRegistry) GetGeneratedRoutes(controller string) []RouteInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	routes := r.routes[controller]
	// Return a copy to prevent external mutation
	result := make([]RouteInfo, len(routes))
	copy(result, routes)
	return result
}

// HasGeneratedRoutes checks if a controller has registered generated routes.
func (r *RouteRegistry) HasGeneratedRoutes(controller string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.routes[controller]) > 0
}

// GetRoutesForController retrieves all routes for a controller.
func (r *RouteRegistry) GetRoutesForController(controller string) ([]RouteInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	routes, ok := r.routes[controller]
	if !ok || len(routes) == 0 {
		return nil, false
	}
	// Return a copy to prevent external mutation
	result := make([]RouteInfo, len(routes))
	copy(result, routes)
	return result, true
}

// AllControllersHaveRoutes checks if any controller has registered routes.
func (r *RouteRegistry) AllControllersHaveRoutes() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.routes) > 0
}

// RegisterGeneratedErrorHandlers registers error handlers.
// This is called by generated code during app initialization.
func (r *ErrorHandlerRegistry) RegisterGeneratedErrorHandlers(handlers ...ErrorHandlerInfo) error {
	if len(handlers) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, handler := range handlers {
		if handler.ErrorType == "" {
			return fmt.Errorf("web/internal: register generated error handler: empty error type")
		}
		if handler.Handler == nil {
			return fmt.Errorf("web/internal: register generated error handler for %s: nil handler", handler.ErrorType)
		}
	}

	for _, handler := range handlers {
		r.handlers[handler.ErrorType] = handler
	}
	slog.Debug("registered generated error handlers", "count", len(handlers))
	return nil
}

// GetGeneratedErrorHandler retrieves an error handler from the registry.
func (r *ErrorHandlerRegistry) GetGeneratedErrorHandler(errorType string) (ErrorHandlerInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.handlers[errorType]
	return handler, ok
}

// HasGeneratedErrorHandlers checks if any error handlers are registered.
func (r *ErrorHandlerRegistry) HasGeneratedErrorHandlers() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers) > 0
}

// GetErrorHandlersForHandler retrieves all error handlers for a handler.
// This returns a slice since a handler can have multiple methods.
func (r *ErrorHandlerRegistry) GetErrorHandlersForHandler(handlerName string) ([]ErrorHandlerInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// For now, return all handlers (they're shared globally)
	// This can be enhanced later if we need per-handler scoping
	if len(r.handlers) == 0 {
		return nil, false
	}
	result := make([]ErrorHandlerInfo, 0, len(r.handlers))
	for _, handler := range r.handlers {
		result = append(result, handler)
	}
	return result, true
}

// GlobalRouteRegistry returns the global route registry instance.
func GlobalRouteRegistry() *RouteRegistry {
	return globalRouteRegistry
}

// GlobalErrorHandlerRegistry returns the global error handler registry instance.
func GlobalErrorHandlerRegistry() *ErrorHandlerRegistry {
	return globalErrorHandlerRegistry
}
