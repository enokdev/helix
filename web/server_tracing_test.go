package web_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/enokdev/helix/web"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// newTestTracerProvider returns a TracerProvider that records spans in memory
// and a SpanRecorder for assertions.
func newTestTracerProvider(t *testing.T) (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	return tp, recorder
}

func TestWithTracerProvider_NilIsNoop(t *testing.T) {
	// A nil TracerProvider must not panic and must not install any middleware.
	srv := web.NewServer(web.WithTracerProvider(nil))
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	if err := srv.RegisterRoute(http.MethodGet, "/health", func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		return nil
	}); err != nil {
		t.Fatalf("RegisterRoute: %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	resp, err := srv.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestWithTracerProvider_CreatesSpanForRequest(t *testing.T) {
	tp, recorder := newTestTracerProvider(t)

	srv := web.NewServer(web.WithTracerProvider(tp))
	if err := srv.RegisterRoute(http.MethodGet, "/ping", func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"ok": "true"})
	}); err != nil {
		t.Fatalf("RegisterRoute: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	if _, err := srv.ServeHTTP(req); err != nil {
		t.Fatalf("ServeHTTP: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatal("expected at least one span, got none")
	}

	span := spans[0]
	if span.Name() != "GET /ping" {
		t.Errorf("expected span name %q, got %q", "GET /ping", span.Name())
	}
}

func TestWithTracerProvider_SpanNameUsesRoutePath(t *testing.T) {
	// The span name must use the parametrised route path, not the raw URL.
	tp, recorder := newTestTracerProvider(t)

	srv := web.NewServer(web.WithTracerProvider(tp))
	if err := srv.RegisterRoute(http.MethodGet, "/users/:id", func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		return nil
	}); err != nil {
		t.Fatalf("RegisterRoute: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/users/42", nil)
	if _, err := srv.ServeHTTP(req); err != nil {
		t.Fatalf("ServeHTTP: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	// The span name should contain the parametrised path, not the raw URL /users/42.
	name := spans[0].Name()
	if name == "GET /users/42" {
		t.Errorf("span name must use route pattern, not raw URL: got %q", name)
	}
	if name != "GET /users/:id" {
		t.Errorf("expected span name %q, got %q", "GET /users/:id", name)
	}
}

func TestWithTracerProvider_PropagatesTraceparentInResponse(t *testing.T) {
	tp, _ := newTestTracerProvider(t)

	srv := web.NewServer(web.WithTracerProvider(tp))
	if err := srv.RegisterRoute(http.MethodGet, "/trace", func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		return nil
	}); err != nil {
		t.Fatalf("RegisterRoute: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/trace", nil)
	resp, err := srv.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP: %v", err)
	}

	traceparent := resp.Header.Get("Traceparent")
	if traceparent == "" {
		t.Error("expected Traceparent header in response, got empty")
	}
}

func TestWithTracerProvider_ExtractsIncomingTraceparent(t *testing.T) {
	// A request with a traceparent header should produce a child span (same trace ID).
	tp, recorder := newTestTracerProvider(t)

	srv := web.NewServer(web.WithTracerProvider(tp))
	if err := srv.RegisterRoute(http.MethodGet, "/child", func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		return nil
	}); err != nil {
		t.Fatalf("RegisterRoute: %v", err)
	}

	// Build a valid traceparent header for a synthetic parent span.
	// Format: 00-<trace-id>-<parent-span-id>-<flags>
	parentTraceparent := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

	req, _ := http.NewRequest(http.MethodGet, "/child", nil)
	req.Header.Set("Traceparent", parentTraceparent)

	if _, err := srv.ServeHTTP(req); err != nil {
		t.Fatalf("ServeHTTP: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	span := spans[0]
	// The span must share the parent's trace ID.
	traceID := span.SpanContext().TraceID().String()
	if !strings.Contains(parentTraceparent, traceID) {
		t.Errorf("expected child span to share parent trace ID %q, got span trace ID %q",
			"0af7651916cd43dd8448eb211c80319c", traceID)
	}
}

func TestWithTracerProvider_NoSpansWhenDisabled(t *testing.T) {
	// Without WithTracerProvider, no spans should be created via a stdouttrace exporter.
	var buf bytes.Buffer
	exp, err := stdouttrace.New(stdouttrace.WithWriter(&buf))
	if err != nil {
		t.Fatalf("stdouttrace.New: %v", err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	_ = tp // not passed to NewServer

	srv := web.NewServer() // no WithTracerProvider
	if err := srv.RegisterRoute(http.MethodGet, "/noop", func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		return nil
	}); err != nil {
		t.Fatalf("RegisterRoute: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/noop", nil)
	if _, err := srv.ServeHTTP(req); err != nil {
		t.Fatalf("ServeHTTP: %v", err)
	}

	if buf.Len() > 0 {
		t.Errorf("expected no span output when tracing not installed, got: %s", buf.String())
	}
}
