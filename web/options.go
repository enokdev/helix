package web

type serverOptions struct {
	routeObserver RouteObserver
}

// Option configures an HTTP server.
type Option func(*serverOptions)

// WithRouteObserver installs a RouteObserver that receives an observation for
// every HTTP request handled by the server.
// A nil or typed-nil observer is silently ignored.
func WithRouteObserver(observer RouteObserver) Option {
	return func(o *serverOptions) {
		if observer == nil || isNilValue(observer) {
			return
		}
		o.routeObserver = observer
	}
}
