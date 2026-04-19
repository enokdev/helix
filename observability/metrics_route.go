package observability

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/enokdev/helix/web"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsPath = "/actuator/metrics"

// MetricsRouteOption configures the metrics route registration.
type MetricsRouteOption func(*metricsRouteOptions)

type metricsRouteOptions struct {
	guard    web.Guard
	hasGuard bool
}

// WithMetricsGuard installs an optional guard on /actuator/metrics.
// By default the endpoint is public. Pass a guard to restrict access.
// A nil or typed-nil guard returns ErrInvalidMetrics at registration time.
func WithMetricsGuard(guard web.Guard) MetricsRouteOption {
	return func(o *metricsRouteOptions) {
		o.guard = guard
		o.hasGuard = true
	}
}

// RegisterMetricsRoute registers GET /actuator/metrics on server using the
// provided Prometheus registry. The response uses the standard Prometheus text
// exposition format (text/plain).
//
// An optional guard can be supplied via WithMetricsGuard; by default the
// endpoint is public.
func RegisterMetricsRoute(server web.HTTPServer, registry *prometheus.Registry, opts ...MetricsRouteOption) error {
	if server == nil || isNilInterface(server) {
		return fmt.Errorf("observability: register metrics route: server: %w", ErrInvalidMetrics)
	}
	if registry == nil {
		return fmt.Errorf("observability: register metrics route: registry: %w", ErrInvalidMetrics)
	}

	options := &metricsRouteOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(options)
		}
	}

	if options.hasGuard && (options.guard == nil || isNilInterface(options.guard)) {
		return fmt.Errorf("observability: register metrics route: guard: %w", ErrInvalidMetrics)
	}

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	if err := server.RegisterRoute(http.MethodGet, metricsPath, func(ctx web.Context) error {
		if options.guard != nil {
			if err := options.guard.CanActivate(ctx); err != nil {
				return err
			}
		}
		return servePrometheus(ctx, handler)
	}); err != nil {
		return fmt.Errorf("observability: register metrics route %s: %w", metricsPath, err)
	}

	return nil
}

// servePrometheus bridges the net/http promhttp handler to web.Context without
// importing Fiber. A minimal net/http request and response recorder are used to
// capture the Prometheus output, which is then forwarded to ctx.
func servePrometheus(ctx web.Context, handler http.Handler) error {
	req, err := http.NewRequest(ctx.Method(), ctx.OriginalURL(), nil)
	if err != nil {
		return fmt.Errorf("observability: build prometheus request: %w", err)
	}
	if accept := ctx.Header("Accept"); accept != "" {
		req.Header.Set("Accept", accept)
	}
	if enc := ctx.Header("Accept-Encoding"); enc != "" {
		req.Header.Set("Accept-Encoding", enc)
	}

	rec := &responseRecorder{
		header: make(http.Header),
		buf:    &bytes.Buffer{},
		code:   http.StatusOK,
	}
	handler.ServeHTTP(rec, req)

	ctx.Status(rec.code)
	for key, values := range rec.header {
		for _, v := range values {
			ctx.SetHeader(key, v)
		}
	}
	return ctx.Send(rec.buf.Bytes())
}

// responseRecorder is a minimal http.ResponseWriter that buffers the response
// produced by promhttp.HandlerFor.
type responseRecorder struct {
	header      http.Header
	buf         *bytes.Buffer
	code        int
	wroteHeader bool
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.code = code
	r.wroteHeader = true
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.buf.Write(b)
}
