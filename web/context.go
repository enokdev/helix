package web

// Context exposes request data to Helix handlers without leaking the
// underlying HTTP framework context.
type Context interface {
	Param(key string) string
	Header(key string) string
	IP() string
}
