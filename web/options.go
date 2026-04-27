package web

import "go.opentelemetry.io/otel/trace"

type serverOptions struct {
	routeObserver  RouteObserver
	tracerProvider trace.TracerProvider
	generatedOnly  bool
}

// Option configures an HTTP server.
type Option func(*serverOptions)

// WithGeneratedOnly disables AST-parsing fallback for route and error handler
// discovery, requiring all directives to be pre-generated.
func WithGeneratedOnly() Option {
	return func(o *serverOptions) {
		o.generatedOnly = true
	}
}

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

// WithTracerProvider installs an OpenTelemetry TracerProvider that
// automatically creates a span for every incoming HTTP request and propagates
// the W3C trace context via traceparent/tracestate headers.
// A nil provider is silently ignored — tracing is disabled.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(o *serverOptions) {
		if tp == nil || isNilValue(tp) {
			return
		}
		o.tracerProvider = tp
	}
}
