package web

import (
	"fmt"
	"net/http"
)

const (
	codeUnauthorized = "UNAUTHORIZED"
	codeForbidden    = "FORBIDDEN"
)

// Guard decides whether a request can continue to its handler.
type Guard interface {
	CanActivate(Context) error
}

// GuardFunc adapts a function to Guard.
type GuardFunc func(Context) error

// CanActivate runs f for a request.
func (f GuardFunc) CanActivate(ctx Context) error {
	return f(ctx)
}

// GuardFactory creates a guard from a directive argument.
type GuardFactory func(argument string) (Guard, error)

type guardRegistrar interface {
	registerGuard(name string, guard Guard) error
	registerGuardFactory(name string, factory GuardFactory) error
}

type accessError struct {
	status int
	detail ErrorDetail
}

// Unauthorized returns a structured 401 error for guards.
func Unauthorized(message string) error {
	return newAccessError(http.StatusUnauthorized, "UnauthorizedError", codeUnauthorized, message)
}

// Forbidden returns a structured 403 error for guards.
func Forbidden(message string) error {
	return newAccessError(http.StatusForbidden, "ForbiddenError", codeForbidden, message)
}

func newAccessError(status int, errorType, code, message string) *accessError {
	if message == "" {
		message = http.StatusText(status)
	}
	return &accessError{
		status: status,
		detail: ErrorDetail{
			Type:    errorType,
			Message: message,
			Code:    code,
		},
	}
}

func (e *accessError) Error() string {
	return e.detail.Message
}

func (e *accessError) StatusCode() int {
	return e.status
}

func (e *accessError) ErrorType() string {
	return e.detail.Type
}

func (e *accessError) ErrorCode() string {
	return e.detail.Code
}

func (e *accessError) ErrorField() string {
	return e.detail.Field
}

// RegisterGuard registers a named guard on a Helix server.
func RegisterGuard(server HTTPServer, name string, guard Guard) error {
	registrar, ok := server.(guardRegistrar)
	if !ok || registrar == nil {
		return fmt.Errorf("web: register guard: %w", ErrInvalidDirective)
	}
	if err := registrar.registerGuard(name, guard); err != nil {
		return fmt.Errorf("web: register guard %s: %w", name, err)
	}
	return nil
}

// RegisterGuardFactory registers a named guard factory on a Helix server.
func RegisterGuardFactory(server HTTPServer, name string, factory GuardFactory) error {
	registrar, ok := server.(guardRegistrar)
	if !ok || registrar == nil {
		return fmt.Errorf("web: register guard factory: %w", ErrInvalidDirective)
	}
	if err := registrar.registerGuardFactory(name, factory); err != nil {
		return fmt.Errorf("web: register guard factory %s: %w", name, err)
	}
	return nil
}
