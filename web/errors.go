package web

import "errors"

// ErrInvalidRoute is returned when a route cannot be registered.
var ErrInvalidRoute = errors.New("web: invalid route")
