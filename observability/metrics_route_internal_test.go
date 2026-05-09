package observability

import (
	"context"
	"net/http"
	"testing"
)

func TestServePrometheusPreservesMultiValueHeaders(t *testing.T) {
	t.Parallel()

	ctx := newMetricsRouteTestContext()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("X-Prometheus-Warning", "first")
		w.Header().Add("X-Prometheus-Warning", "second")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	if err := servePrometheus(ctx, handler); err != nil {
		t.Fatalf("servePrometheus() error = %v", err)
	}

	values := ctx.headers.Values("X-Prometheus-Warning")
	if len(values) != 2 || values[0] != "first" || values[1] != "second" {
		t.Fatalf("X-Prometheus-Warning values = %#v, want [first second]", values)
	}
}

type metricsRouteTestContext struct {
	headers http.Header
	status  int
}

func newMetricsRouteTestContext() *metricsRouteTestContext {
	return &metricsRouteTestContext{headers: make(http.Header)}
}

func (c *metricsRouteTestContext) Method() string            { return http.MethodGet }
func (c *metricsRouteTestContext) Path() string              { return metricsPath }
func (c *metricsRouteTestContext) OriginalURL() string       { return metricsPath }
func (c *metricsRouteTestContext) Param(string) string       { return "" }
func (c *metricsRouteTestContext) Query(string) string       { return "" }
func (c *metricsRouteTestContext) Header(string) string      { return "" }
func (c *metricsRouteTestContext) IP() string                { return "" }
func (c *metricsRouteTestContext) Body() []byte              { return nil }
func (c *metricsRouteTestContext) Status(code int)           { c.status = code }
func (c *metricsRouteTestContext) SetHeader(k, v string)     { c.headers.Set(k, v) }
func (c *metricsRouteTestContext) AppendHeader(k, v string)  { c.headers.Add(k, v) }
func (c *metricsRouteTestContext) Send([]byte) error         { return nil }
func (c *metricsRouteTestContext) JSON(any) error            { return nil }
func (c *metricsRouteTestContext) Context() context.Context  { return context.Background() }
func (c *metricsRouteTestContext) Locals(string, ...any) any { return nil }
