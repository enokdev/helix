package web

import "errors"

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

// ErrorResponse is the structured JSON error envelope returned by Helix.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
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
	status int
	detail ErrorDetail
}

func newRequestError(status int, code, field, message string) *RequestError {
	return &RequestError{
		status: status,
		detail: ErrorDetail{
			Type:    requestErrorType,
			Message: message,
			Field:   field,
			Code:    code,
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
