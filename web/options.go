package web

type serverOptions struct{}

// Option configures an HTTP server.
type Option func(*serverOptions)
