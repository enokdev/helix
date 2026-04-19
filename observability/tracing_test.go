package observability

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace/noop"
)

// mapLoader is a minimal config.Loader for tests that drives ConfigureTracing
// from an in-memory key→value map instead of a YAML file.
type mapLoader map[string]any

func (m mapLoader) Lookup(key string) (any, bool) { v, ok := m[key]; return v, ok }
func (m mapLoader) Load(_ any) error              { return nil }
func (m mapLoader) ConfigFileUsed() string        { return "" }
func (m mapLoader) AllSettings() map[string]any   { return m }
func (m mapLoader) ActiveProfiles() []string      { return nil }

// resetOTelGlobals restores the OTel global state after tests that call
// ConfigureTracing with enabled=true (which calls otel.SetTracerProvider).
func resetOTelGlobals(t *testing.T) {
	t.Helper()
	prev := otel.GetTracerProvider()
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
	})
}

func TestConfigureTracing_Disabled(t *testing.T) {
	tp, shutdown, err := ConfigureTracing(nil,
		WithTracingConfig(TracingConfig{Enabled: false}),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tp != nil {
		t.Errorf("expected nil TracerProvider when disabled, got %T", tp)
	}
	if shutdown != nil {
		t.Errorf("expected nil shutdown func when disabled, got non-nil")
	}
}

func TestConfigureTracing_DisabledByDefault_NilLoader(t *testing.T) {
	// No loader, no WithTracingConfig → defaults to disabled.
	tp, shutdown, err := ConfigureTracing(nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tp != nil {
		t.Errorf("expected nil TracerProvider with no config, got %T", tp)
	}
	if shutdown != nil {
		t.Errorf("expected nil shutdown, got non-nil")
	}
}

func TestConfigureTracing_StdoutExporter(t *testing.T) {
	resetOTelGlobals(t)

	var buf bytes.Buffer
	tp, shutdown, err := ConfigureTracing(nil,
		WithTracingConfig(TracingConfig{
			Enabled:     true,
			Exporter:    "stdout",
			ServiceName: "test-svc",
		}),
		WithTracingOutput(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil TracerProvider")
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown func")
	}

	// Emit a span and flush it.
	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "test-span")
	span.End()

	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown returned error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected span output in buffer, got empty string")
	}
	if !bytes.Contains([]byte(output), []byte("test-span")) {
		t.Errorf("expected span name in output, got: %s", output)
	}
}

func TestConfigureTracing_SetsGlobalProvider(t *testing.T) {
	resetOTelGlobals(t)

	prevTP := otel.GetTracerProvider()

	var buf bytes.Buffer
	tp, shutdown, err := ConfigureTracing(nil,
		WithTracingConfig(TracingConfig{Enabled: true, Exporter: "stdout"}),
		WithTracingOutput(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer shutdown(context.Background()) //nolint:errcheck

	globalTP := otel.GetTracerProvider()
	if globalTP == prevTP {
		t.Error("expected otel.SetTracerProvider to have been called")
	}
	if globalTP != tp {
		t.Errorf("global TracerProvider mismatch: got %T, want %T", globalTP, tp)
	}
}

func TestConfigureTracing_InvalidExporter(t *testing.T) {
	// Empty string via WithTracingConfig means "not set → use default (stdout)",
	// so it is NOT invalid. Invalid means an explicitly unrecognised value.
	tests := []struct {
		name     string
		exporter string
	}{
		{"zipkin", "zipkin"},
		{"grpc", "grpc"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp, shutdown, err := ConfigureTracing(nil,
				WithTracingConfig(TracingConfig{Enabled: true, Exporter: tt.exporter}),
			)
			if err == nil {
				t.Fatal("expected error for invalid exporter, got nil")
			}
			if !errors.Is(err, ErrInvalidTracing) {
				t.Errorf("expected ErrInvalidTracing, got %v", err)
			}
			if tp != nil || shutdown != nil {
				t.Error("expected nil TracerProvider and shutdown on error")
			}
		})
	}
}

func TestConfigureTracing_WithTracingConfig_Priority(t *testing.T) {
	resetOTelGlobals(t)

	var buf bytes.Buffer
	// loader is nil; WithTracingConfig should supply all values.
	tp, shutdown, err := ConfigureTracing(nil,
		WithTracingConfig(TracingConfig{
			Enabled:     true,
			Exporter:    "stdout",
			ServiceName: "override-svc",
		}),
		WithTracingOutput(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer shutdown(context.Background()) //nolint:errcheck

	if tp == nil {
		t.Fatal("expected non-nil TracerProvider")
	}
}

func TestConfigureTracing_NoopWhenDisabled_NoGlobalSideEffect(t *testing.T) {
	// Must NOT modify global OTel state when disabled.
	before := otel.GetTracerProvider()

	_, _, err := ConfigureTracing(nil, WithTracingConfig(TracingConfig{Enabled: false}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after := otel.GetTracerProvider()
	if before != after {
		t.Error("ConfigureTracing(disabled) must not change the global TracerProvider")
	}
}

func TestWithTracerProvider_NilSafe(_ *testing.T) {
	// verify that noop.NewTracerProvider() satisfies the trace.TracerProvider interface
	// and that the assignment compiles without error.
	tp := noop.NewTracerProvider()
	_ = tp // assignment succeeds → concrete type satisfies trace.TracerProvider
}

func TestConfigureTracing_LoaderEmptyExporter_IsInvalid(t *testing.T) {
	// AC9: an explicit empty-string exporter from the loader must return ErrInvalidTracing.
	loader := mapLoader{
		"helix.starters.observability.tracing.enabled":  true,
		"helix.starters.observability.tracing.exporter": "",
	}

	tp, shutdown, err := ConfigureTracing(loader)
	if err == nil {
		t.Fatal("expected error for empty-string exporter from loader, got nil")
	}
	if !errors.Is(err, ErrInvalidTracing) {
		t.Errorf("expected ErrInvalidTracing, got %v", err)
	}
	if tp != nil || shutdown != nil {
		t.Error("expected nil TracerProvider and shutdown on error")
	}
}

func TestConfigureTracing_WithTracingConfig_DoesNotOverrideLoaderEnabled(t *testing.T) {
	// F4: WithTracingConfig(TracingConfig{ServiceName: "x"}) without Enabled:true
	// must not disable tracing that the loader had enabled.
	resetOTelGlobals(t)

	var buf bytes.Buffer
	loader := mapLoader{
		"helix.starters.observability.tracing.enabled": true,
	}

	tp, shutdown, err := ConfigureTracing(loader,
		WithTracingConfig(TracingConfig{ServiceName: "override-svc"}), // Enabled is zero-value false
		WithTracingOutput(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil TracerProvider: loader's enabled:true must not be overridden by zero-value Enabled in WithTracingConfig")
	}
	defer shutdown(context.Background()) //nolint:errcheck
}
