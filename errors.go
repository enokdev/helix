package helix

import (
	"errors"
	"net/http"
)

var (
	// ErrInvalidComponent is returned when a value cannot be registered as a Helix component.
	ErrInvalidComponent = errors.New("helix: invalid component")
	// ErrScanRequiresComponents is returned when source scan finds marker types
	// that cannot be instantiated by the runtime bootstrap.
	ErrScanRequiresComponents = errors.New("helix: scan requires runtime components")
)

// NotFoundError represents a missing resource returned by application code.
type NotFoundError struct {
	Message string
}

// Error implements error.
func (e NotFoundError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "resource not found"
}

// StatusCode returns the HTTP status associated with this error.
func (e NotFoundError) StatusCode() int {
	return http.StatusNotFound
}

// ErrorType returns the structured error type name.
func (e NotFoundError) ErrorType() string {
	return "NotFoundError"
}

// ErrorCode returns the machine-readable error code.
func (e NotFoundError) ErrorCode() string {
	return "NOT_FOUND"
}

// ErrorField returns the field associated with this error, when any.
func (e NotFoundError) ErrorField() string {
	return ""
}

// ValidationError represents invalid user input returned by application code.
type ValidationError struct {
	Message string
	Field   string
}

// Error implements error.
func (e ValidationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "validation failed"
}

// StatusCode returns the HTTP status associated with this error.
func (e ValidationError) StatusCode() int {
	return http.StatusBadRequest
}

// ErrorType returns the structured error type name.
func (e ValidationError) ErrorType() string {
	return "ValidationError"
}

// ErrorCode returns the machine-readable error code.
func (e ValidationError) ErrorCode() string {
	return "VALIDATION_FAILED"
}

// ErrorField returns the field associated with this error, when any.
func (e ValidationError) ErrorField() string {
	return e.Field
}
