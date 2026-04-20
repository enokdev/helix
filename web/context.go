package web

// Context exposes request data to Helix handlers without leaking the
// underlying HTTP framework context.
type Context interface {
	Method() string
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
}
