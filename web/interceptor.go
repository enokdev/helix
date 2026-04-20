package web

import "fmt"

// Interceptor wraps request handling around a route handler.
type Interceptor interface {
	Intercept(Context, HandlerFunc) error
}

// InterceptorFunc adapts a function to Interceptor.
type InterceptorFunc func(Context, HandlerFunc) error

// Intercept runs f for a request.
func (f InterceptorFunc) Intercept(ctx Context, next HandlerFunc) error {
	return f(ctx, next)
}

// InterceptorFactory creates an interceptor from a directive argument.
type InterceptorFactory func(argument string) (Interceptor, error)

type interceptorRegistrar interface {
	registerInterceptor(name string, interceptor Interceptor) error
	registerInterceptorFactory(name string, factory InterceptorFactory) error
}

// RegisterInterceptor registers a named interceptor on a Helix server.
func RegisterInterceptor(server HTTPServer, name string, interceptor Interceptor) error {
	registrar, ok := server.(interceptorRegistrar)
	if !ok || registrar == nil {
		return fmt.Errorf("web: register interceptor: %w", ErrInvalidDirective)
	}
	if err := registrar.registerInterceptor(name, interceptor); err != nil {
		return fmt.Errorf("web: register interceptor %s: %w", name, err)
	}
	return nil
}

// RegisterInterceptorFactory registers a named interceptor factory on a Helix server.
func RegisterInterceptorFactory(server HTTPServer, name string, factory InterceptorFactory) error {
	registrar, ok := server.(interceptorRegistrar)
	if !ok || registrar == nil {
		return fmt.Errorf("web: register interceptor factory: %w", ErrInvalidDirective)
	}
	if err := registrar.registerInterceptorFactory(name, factory); err != nil {
		return fmt.Errorf("web: register interceptor factory %s: %w", name, err)
	}
	return nil
}
