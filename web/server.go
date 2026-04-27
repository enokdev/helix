package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"

	fiberinternal "github.com/enokdev/helix/web/internal"
)

// HTTPServer exposes Helix's minimal HTTP server contract.
type HTTPServer interface {
	Start(addr string) error
	Stop(ctx context.Context) error
	RegisterRoute(method, path string, handler HandlerFunc) error
	// ServeHTTP executes a request against the server without starting a
	// network listener. Intended for use in tests and tooling.
	ServeHTTP(req *http.Request) (*http.Response, error)
	// IsGeneratedOnly returns true if the server is configured to only use
	// pre-generated route and error handler metadata.
	IsGeneratedOnly() bool
}

type server struct {
	adapter              fiberinternal.Adapter
	errorHandlers        map[string]errorHandlerInvoker
	errorHandlerOrder    []string
	guards               map[string]Guard
	globalGuards         []Guard
	guardFactories       map[string]GuardFactory
	interceptors         map[string]Interceptor
	interceptorFactories map[string]InterceptorFactory
	cache                *cacheStore
	routeObserver        RouteObserver
	generatedOnly        bool
}

// NewServer creates an HTTP server backed by an internal Fiber adapter.
func NewServer(opts ...Option) HTTPServer {
	options := serverOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	s := &server{
		adapter:              fiberinternal.NewAdapter(fiberinternal.WithTracerProvider(options.tracerProvider)),
		errorHandlers:        make(map[string]errorHandlerInvoker),
		guards:               make(map[string]Guard),
		guardFactories:       make(map[string]GuardFactory),
		interceptors:         make(map[string]Interceptor),
		interceptorFactories: make(map[string]InterceptorFactory),
		cache:                newCacheStore(),
		routeObserver:        options.routeObserver,
		generatedOnly:        options.generatedOnly,
	}
	s.interceptorFactories["cache"] = cacheInterceptorFactory(s.cache)
	return s
}

func (s *server) IsGeneratedOnly() bool {
	return s.generatedOnly
}

func (s *server) Start(addr string) error {
	if err := s.adapter.Start(addr); err != nil {
		return fmt.Errorf("web: start %s: %w", addr, err)
	}
	return nil
}

func (s *server) Stop(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("web: stop: nil context")
	}
	// Stop the cache sweep goroutine gracefully.
	if err := s.cache.Stop(); err != nil {
		return fmt.Errorf("web: stop cache: %w", err)
	}
	if err := s.adapter.Stop(ctx); err != nil {
		return fmt.Errorf("web: stop: %w", err)
	}
	return nil
}

func (s *server) RegisterRoute(method, path string, handler HandlerFunc) error {
	normalizedMethod, err := validateRoute(method, path, handler)
	if err != nil {
		return err
	}

	observer := s.routeObserver
	routePath := path

	err = s.adapter.RegisterRoute(normalizedMethod, path, func(ctx fiberinternal.Context) error {
		start := time.Now()
		observed := &observingContext{Context: ctx}

		// Run global guards before the handler.
		for _, g := range s.globalGuards {
			if guardErr := g.CanActivate(observed); guardErr != nil {
				return writeErrorResponse(observed, guardErr)
			}
		}

		handlerErr := handler(observed)
		if handlerErr != nil {
			if handled, handleErr := s.writeRegisteredError(observed, handlerErr); handled {
				handlerErr = handleErr
			} else {
				handlerErr = writeErrorResponse(observed, handlerErr)
			}
		}

		if observer != nil {
			statusCode := observed.statusCode
			if statusCode == 0 {
				statusCode = http.StatusOK
			}
			observer.Observe(RouteObservation{
				Method:     ctx.Method(),
				Route:      routePath,
				StatusCode: statusCode,
				Duration:   time.Since(start),
			})
		}

		return handlerErr
	})
	if err != nil {
		return fmt.Errorf("web: register route %s %s: %w", normalizedMethod, path, err)
	}
	return nil
}

// observingContext wraps a Context to intercept Status calls and record the
// final status code set during handler execution.
type observingContext struct {
	fiberinternal.Context
	statusCode int // 0 means no explicit Status call; interpret as 200
}

func (o *observingContext) Status(code int) {
	o.statusCode = code
	o.Context.Status(code)
}

func (o *observingContext) SetHeader(key, value string) {
	o.Context.SetHeader(key, value)
}

func (o *observingContext) Send(body []byte) error {
	return o.Context.Send(body)
}

func (s *server) registerErrorHandler(handler any) error {
	handlers, err := buildErrorHandlers(s, handler)
	if err != nil {
		return err
	}

	for errorType := range handlers {
		if _, exists := s.errorHandlers[errorType]; exists {
			return fmt.Errorf("web: register error handler duplicate %s: %w", errorType, ErrInvalidErrorHandler)
		}
	}
	errorTypes := make([]string, 0, len(handlers))
	for errorType := range handlers {
		errorTypes = append(errorTypes, errorType)
	}
	sort.Strings(errorTypes)

	for _, errorType := range errorTypes {
		s.errorHandlers[errorType] = handlers[errorType]
		s.errorHandlerOrder = append(s.errorHandlerOrder, errorType)
	}
	return nil
}

func (s *server) registerGuard(name string, guard Guard) error {
	name = strings.TrimSpace(name)
	if name == "" || guard == nil || isNilValue(guard) {
		return fmt.Errorf("web: validate guard %q: %w", name, ErrInvalidDirective)
	}
	if _, exists := s.guards[name]; exists {
		return fmt.Errorf("web: validate guard duplicate %s: %w", name, ErrInvalidDirective)
	}
	if _, exists := s.guardFactories[name]; exists {
		return fmt.Errorf("web: validate guard duplicate %s: %w", name, ErrInvalidDirective)
	}
	s.guards[name] = guard
	return nil
}

func (s *server) addGlobalGuard(guard Guard) {
	s.globalGuards = append(s.globalGuards, guard)
}

func (s *server) registerGuardFactory(name string, factory GuardFactory) error {
	name = strings.TrimSpace(name)
	if name == "" || factory == nil {
		return fmt.Errorf("web: validate guard factory %q: %w", name, ErrInvalidDirective)
	}
	if _, exists := s.guards[name]; exists {
		return fmt.Errorf("web: validate guard duplicate %s: %w", name, ErrInvalidDirective)
	}
	if _, exists := s.guardFactories[name]; exists {
		return fmt.Errorf("web: validate guard duplicate %s: %w", name, ErrInvalidDirective)
	}
	s.guardFactories[name] = factory
	return nil
}

func (s *server) registerInterceptor(name string, interceptor Interceptor) error {
	name = strings.TrimSpace(name)
	if name == "" || interceptor == nil || isNilValue(interceptor) {
		return fmt.Errorf("web: validate interceptor %q: %w", name, ErrInvalidDirective)
	}
	if _, exists := s.interceptors[name]; exists {
		return fmt.Errorf("web: validate interceptor duplicate %s: %w", name, ErrInvalidDirective)
	}
	if _, exists := s.interceptorFactories[name]; exists {
		return fmt.Errorf("web: validate interceptor duplicate %s: %w", name, ErrInvalidDirective)
	}
	s.interceptors[name] = interceptor
	return nil
}

func (s *server) registerInterceptorFactory(name string, factory InterceptorFactory) error {
	name = strings.TrimSpace(name)
	if name == "" || factory == nil {
		return fmt.Errorf("web: validate interceptor factory %q: %w", name, ErrInvalidDirective)
	}
	if _, exists := s.interceptors[name]; exists {
		return fmt.Errorf("web: validate interceptor duplicate %s: %w", name, ErrInvalidDirective)
	}
	if _, exists := s.interceptorFactories[name]; exists {
		return fmt.Errorf("web: validate interceptor duplicate %s: %w", name, ErrInvalidDirective)
	}
	s.interceptorFactories[name] = factory
	return nil
}

func (s *server) resolveGuard(directive namedDirective) (Guard, error) {
	if directive.argument == "" {
		guard, ok := s.guards[directive.name]
		if !ok {
			return nil, fmt.Errorf("web: resolve guard %s: %w", directive.raw, ErrInvalidDirective)
		}
		return guard, nil
	}
	factory, ok := s.guardFactories[directive.name]
	if !ok {
		return nil, fmt.Errorf("web: resolve guard %s: %w", directive.raw, ErrInvalidDirective)
	}
	guard, err := factory(directive.argument)
	if err != nil {
		return nil, fmt.Errorf("web: resolve guard %s: %w", directive.raw, errors.Join(err, ErrInvalidDirective))
	}
	if guard == nil || isNilValue(guard) {
		return nil, fmt.Errorf("web: resolve guard %s: %w", directive.raw, ErrInvalidDirective)
	}
	return guard, nil
}

func (s *server) resolveInterceptor(directive namedDirective) (Interceptor, error) {
	if directive.argument == "" {
		interceptor, ok := s.interceptors[directive.name]
		if !ok {
			if _, isFactory := s.interceptorFactories[directive.name]; isFactory {
				return nil, fmt.Errorf("web: resolve interceptor %s: %q requires an argument (use %s:<value>): %w", directive.raw, directive.name, directive.name, ErrInvalidDirective)
			}
			return nil, fmt.Errorf("web: resolve interceptor %s: %w", directive.raw, ErrInvalidDirective)
		}
		return interceptor, nil
	}
	factory, ok := s.interceptorFactories[directive.name]
	if !ok {
		return nil, fmt.Errorf("web: resolve interceptor %s: %w", directive.raw, ErrInvalidDirective)
	}
	interceptor, err := factory(directive.argument)
	if err != nil {
		return nil, fmt.Errorf("web: resolve interceptor %s: %w", directive.raw, errors.Join(err, ErrInvalidDirective))
	}
	if interceptor == nil || isNilValue(interceptor) {
		return nil, fmt.Errorf("web: resolve interceptor %s: %w", directive.raw, ErrInvalidDirective)
	}
	return interceptor, nil
}

func isNilValue(value any) bool {
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func (s *server) writeRegisteredError(ctx Context, err error) (bool, error) {
	for _, errorType := range s.errorHandlerOrder {
		handler := s.errorHandlers[errorType]
		if handled, writeErr := handler(ctx, err); handled {
			return true, writeErr
		}
	}
	return false, nil
}

func (s *server) ServeHTTP(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("web: serve http: nil request")
	}
	resp, err := s.adapter.ServeHTTP(req)
	if err != nil {
		return nil, fmt.Errorf("web: serve http: %w", err)
	}
	return resp, nil
}

func validateRoute(method, path string, handler HandlerFunc) (string, error) {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	if !isSupportedMethod(normalizedMethod) {
		return "", fmt.Errorf("web: validate route method %q: %w", method, ErrInvalidRoute)
	}
	if path == "" || !strings.HasPrefix(path, "/") || strings.ContainsAny(path, " \t\r\n") {
		return "", fmt.Errorf("web: validate route path %q: %w", path, ErrInvalidRoute)
	}
	if handler == nil {
		return "", fmt.Errorf("web: validate route handler: %w", ErrInvalidRoute)
	}
	return normalizedMethod, nil
}

func isSupportedMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
