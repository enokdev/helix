package web

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

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
}

type server struct {
	adapter           fiberinternal.Adapter
	errorHandlers     map[string]errorHandlerInvoker
	errorHandlerOrder []string
}

// NewServer creates an HTTP server backed by an internal Fiber adapter.
func NewServer(opts ...Option) HTTPServer {
	options := serverOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	return &server{
		adapter:       fiberinternal.NewAdapter(),
		errorHandlers: make(map[string]errorHandlerInvoker),
	}
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

	err = s.adapter.RegisterRoute(normalizedMethod, path, func(ctx fiberinternal.Context) error {
		if err := handler(ctx); err != nil {
			if handled, handleErr := s.writeRegisteredError(ctx, err); handled {
				return handleErr
			}
			return writeErrorResponse(ctx, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("web: register route %s %s: %w", normalizedMethod, path, err)
	}
	return nil
}

func (s *server) registerErrorHandler(handler any) error {
	handlers, err := buildErrorHandlers(handler)
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
