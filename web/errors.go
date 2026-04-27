package web

import (
	"errors"
	"net/http"
)

// ErrInvalidRoute is returned when a route cannot be registered.
var ErrInvalidRoute = errors.New("web: invalid route")

// ErrInvalidController is returned when a controller cannot be registered.
var ErrInvalidController = errors.New("web: invalid controller")

// ErrUnsupportedHandler is returned when a controller method signature cannot
// be adapted to HandlerFunc.
var ErrUnsupportedHandler = errors.New("web: unsupported handler")

// ErrInvalidDirective is returned when a Helix directive comment cannot be
// parsed.
var ErrInvalidDirective = errors.New("web: invalid directive")

// ErrInvalidRequest is returned when request binding or validation fails before
// a controller handler can be called.
var ErrInvalidRequest = errors.New("web: invalid request")

// ErrInvalidErrorHandler is returned when a centralized error handler cannot
// be registered.
var ErrInvalidErrorHandler = errors.New("web: invalid error handler")

// ErrorResponse is the structured JSON error envelope returned by Helix.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ValidationErrorResponse is the JSON envelope for multiple validation errors.
type ValidationErrorResponse struct {
	Errors []FieldError `json:"errors"`
}

// FieldError represents a single validation error for a field.
type FieldError struct {
	Field string `json:"field"`
	Msg   string `json:"msg"`
}

// ErrorDetail contains a single machine-readable request error.
type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Field   string `json:"field"`
	Code    string `json:"code"`
}

// RequestError carries an HTTP status and structured response for request
// binding failures. It is intentionally small until the central error handler
// story introduces global customization.
type RequestError struct {
	status            int
	detail            ErrorDetail
	validationErrors  []FieldError
	isMultiFieldError bool
}

func newRequestError(status int, code, field, message string) *RequestError {
	return &RequestError{
		status:            status,
		isMultiFieldError: false,
		detail: ErrorDetail{
			Type:    requestErrorType,
			Message: message,
			Field:   field,
			Code:    code,
		},
	}
}

// newMultiFieldValidationError creates a RequestError with multiple validation errors.
func newMultiFieldValidationError(validationErrors []FieldError) *RequestError {
	return &RequestError{
		status:            http.StatusBadRequest,
		isMultiFieldError: true,
		validationErrors:  validationErrors,
		detail: ErrorDetail{
			Type:    requestErrorType,
			Code:    codeValidationFailed,
			Message: "validation failed",
		},
	}
}

// Error implements error.
func (e *RequestError) Error() string {
	return e.detail.Message
}

// Unwrap keeps request errors compatible with errors.Is(err, ErrInvalidRequest).
func (e *RequestError) Unwrap() error {
	return ErrInvalidRequest
}

// StatusCode returns the HTTP status code to write.
func (e *RequestError) StatusCode() int {
	return e.status
}

// ResponseBody returns the JSON body to encode for this error.
func (e *RequestError) ResponseBody() any {
	if e.isMultiFieldError && len(e.validationErrors) > 0 {
		// Multi-field validation errors use the new "errors" array format
		return ValidationErrorResponse{Errors: e.validationErrors}
	}
	// Single error keeps the old format for backward compatibility
	return ErrorResponse{Error: e.detail}
}

// ErrorType returns the structured error type name.
func (e *RequestError) ErrorType() string {
	return e.detail.Type
}

// ErrorCode returns the machine-readable error code.
func (e *RequestError) ErrorCode() string {
	return e.detail.Code
}

// ErrorField returns the field associated with this error, when any.
func (e *RequestError) ErrorField() string {
	return e.detail.Field
}
