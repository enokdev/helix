package observability

import (
	"fmt"
	"sync"

	"github.com/enokdev/helix/web"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	globalRegistryOnce sync.Once
	globalRegistry     *prometheus.Registry
)

// Registry returns the singleton Helix Prometheus registry.
// Use NewRegistry() in tests to obtain an isolated registry.
func Registry() *prometheus.Registry {
	globalRegistryOnce.Do(func() {
		globalRegistry = prometheus.NewRegistry()
	})
	return globalRegistry
}

// NewRegistry creates a fresh, isolated Prometheus registry.
// Prefer this in tests to avoid cross-test metric collisions.
func NewRegistry() (*prometheus.Registry, error) {
	return prometheus.NewRegistry(), nil
}

// httpMetricsObserver implements web.RouteObserver and records request counts
// and durations into Prometheus counters and histograms.
type httpMetricsObserver struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewHTTPMetricsObserver creates a RouteObserver that records HTTP metrics
// into registry. The two collectors are registered immediately; an error is
// returned if registration fails (e.g., duplicate registration).
func NewHTTPMetricsObserver(registry *prometheus.Registry) (web.RouteObserver, error) {
	if registry == nil {
		return nil, fmt.Errorf("observability: new http metrics observer: registry: %w", ErrInvalidMetrics)
	}

	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "helix_http_requests_total",
		Help: "Total number of HTTP requests handled by Helix.",
	}, []string{"method", "route", "status"})

	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "helix_http_request_duration_seconds",
		Help:    "Duration of HTTP requests handled by Helix.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"method", "route", "status"})

	if err := registry.Register(requests); err != nil {
		return nil, fmt.Errorf("observability: register helix_http_requests_total: %w", err)
	}
	if err := registry.Register(duration); err != nil {
		_ = registry.Unregister(requests)
		return nil, fmt.Errorf("observability: register helix_http_request_duration_seconds: %w", err)
	}

	return &httpMetricsObserver{
		requests: requests,
		duration: duration,
	}, nil
}

// Observe records the HTTP request observation as Prometheus metrics.
func (o *httpMetricsObserver) Observe(obs web.RouteObservation) {
	status := fmt.Sprintf("%d", obs.StatusCode)
	labels := prometheus.Labels{
		"method": obs.Method,
		"route":  obs.Route,
		"status": status,
	}
	o.requests.With(labels).Inc()
	o.duration.With(labels).Observe(obs.Duration.Seconds())
}
