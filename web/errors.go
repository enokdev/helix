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
