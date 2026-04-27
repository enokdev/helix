package web

import "context"

// Context exposes request data to Helix handlers without leaking the
// underlying HTTP framework context.
type Context interface {
	Method() string
	Path() string
	OriginalURL() string
	Param(key string) string
	Query(key string) string
	Header(key string) string
	IP() string
	Body() []byte
	Status(code int)
	SetHeader(key, value string)
	Send(body []byte) error
	JSON(body any) error
	// Context returns the request context.
	Context() context.Context
	// Locals stores or retrieves a request-scoped value by key.
	// Call with one value argument to set; call with no value argument to get.
	Locals(key string, value ...any) any
}
