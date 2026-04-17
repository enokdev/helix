package web

// HandlerFunc handles a request through the Helix HTTP abstraction.
type HandlerFunc func(Context) error
