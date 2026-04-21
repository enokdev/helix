package security

import "errors"

// ErrUnauthorized is returned when a request lacks valid credentials.
var ErrUnauthorized = errors.New("security: unauthorized")

// ErrForbidden is returned when a request is authenticated but lacks permission.
var ErrForbidden = errors.New("security: forbidden")

// ErrTokenExpired is returned when a JWT token has expired.
var ErrTokenExpired = errors.New("security: token expired")

// ErrTokenInvalid is returned when a JWT token cannot be parsed or has an invalid signature.
var ErrTokenInvalid = errors.New("security: token invalid")
