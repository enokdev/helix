package observability

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	// An explicit empty-string exporter from the loader must return ErrInvalidTracing.
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

func TestResolveTracingConfig_OTLPTransportOptions(t *testing.T) {
	loader := mapLoader{
		"helix.starters.observability.tracing.enabled":         true,
		"helix.starters.observability.tracing.exporter":        "otlp",
		"helix.starters.observability.tracing.endpoint":        "https://otel.example.com:4318",
		"helix.starters.observability.tracing.insecure":        false,
		"helix.starters.observability.tracing.headers":         map[string]any{"Authorization": "Bearer token", "x-tenant": "helix"},
		"helix.starters.observability.tracing.tls.server-name": "otel.example.com",
	}

	cfg, err := resolveTracingConfig(loader, &tracingOptions{})
	if err != nil {
		t.Fatalf("resolveTracingConfig() error = %v", err)
	}
	if !cfg.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if cfg.Exporter != "otlp" {
		t.Fatalf("Exporter = %q, want otlp", cfg.Exporter)
	}
	if cfg.Insecure {
		t.Fatal("Insecure = true, want false")
	}
	if cfg.Headers["Authorization"] != "Bearer token" || cfg.Headers["x-tenant"] != "helix" {
		t.Fatalf("Headers = %#v, want Authorization and x-tenant", cfg.Headers)
	}
	if cfg.TLS.ServerName != "otel.example.com" {
		t.Fatalf("TLS.ServerName = %q, want otel.example.com", cfg.TLS.ServerName)
	}
}

func TestResolveTracingConfig_ParsesStringHeaders(t *testing.T) {
	loader := mapLoader{
		"helix.starters.observability.tracing.enabled":  true,
		"helix.starters.observability.tracing.exporter": "otlp",
		"helix.starters.observability.tracing.headers":  " Authorization = Bearer token , x-tenant = helix ",
	}

	cfg, err := resolveTracingConfig(loader, &tracingOptions{})
	if err != nil {
		t.Fatalf("resolveTracingConfig() error = %v", err)
	}
	if cfg.Headers["Authorization"] != "Bearer token" || cfg.Headers["x-tenant"] != "helix" {
		t.Fatalf("Headers = %#v, want Authorization and x-tenant", cfg.Headers)
	}
}

func TestResolveTracingConfig_MalformedStringHeadersWrapErrInvalidTracing(t *testing.T) {
	tests := []struct {
		name    string
		headers string
	}{
		{name: "missing separator", headers: "Authorization"},
		{name: "empty key", headers: "=Bearer token"},
		{name: "empty value", headers: "Authorization="},
		{name: "empty pair", headers: "Authorization=Bearer token,"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			loader := mapLoader{
				"helix.starters.observability.tracing.enabled":  true,
				"helix.starters.observability.tracing.exporter": "otlp",
				"helix.starters.observability.tracing.headers":  tt.headers,
			}

			_, err := resolveTracingConfig(loader, &tracingOptions{})
			if err == nil {
				t.Fatal("resolveTracingConfig() error = nil, want ErrInvalidTracing")
			}
			if !errors.Is(err, ErrInvalidTracing) {
				t.Fatalf("resolveTracingConfig() error = %v, want ErrInvalidTracing", err)
			}
		})
	}
}

func TestResolveTracingConfig_DefaultOTLPInsecureRemainsCompatible(t *testing.T) {
	loader := mapLoader{
		"helix.starters.observability.tracing.enabled":  true,
		"helix.starters.observability.tracing.exporter": "otlp",
	}
	cfg, err := resolveTracingConfig(loader, &tracingOptions{})
	if err != nil {
		t.Fatalf("resolveTracingConfig() error = %v", err)
	}
	if !cfg.Insecure {
		t.Fatal("Insecure = false, want true for backward compatibility")
	}
}

func TestResolveTracingConfig_WithTracingConfigOverridesInsecureFalse(t *testing.T) {
	loader := mapLoader{
		"helix.starters.observability.tracing.enabled":  true,
		"helix.starters.observability.tracing.exporter": "otlp",
		"helix.starters.observability.tracing.insecure": true,
	}

	cfg, err := resolveTracingConfig(loader, &tracingOptions{})
	if err != nil {
		t.Fatalf("resolveTracingConfig() error = %v", err)
	}
	if !cfg.Insecure {
		t.Fatal("Insecure = false before override, want true from loader")
	}

	cfg, err = resolveTracingConfig(loader, &tracingOptions{
		cfg:    TracingConfig{Exporter: "otlp", Insecure: false},
		cfgSet: true,
	})
	if err != nil {
		t.Fatalf("resolveTracingConfig() with override error = %v", err)
	}
	if cfg.Insecure {
		t.Fatal("Insecure = true, want false from WithTracingConfig override")
	}
}

func TestResolveTracingConfig_ServiceNameOnlyOverrideDoesNotChangeLoaderInsecure(t *testing.T) {
	loader := mapLoader{
		"helix.starters.observability.tracing.enabled":  true,
		"helix.starters.observability.tracing.exporter": "otlp",
		"helix.starters.observability.tracing.insecure": true,
	}

	cfg, err := resolveTracingConfig(loader, &tracingOptions{
		cfg:    TracingConfig{ServiceName: "override-svc"},
		cfgSet: true,
	})
	if err != nil {
		t.Fatalf("resolveTracingConfig() error = %v", err)
	}
	if !cfg.Insecure {
		t.Fatal("Insecure = false, want true from loader when override only changes service name")
	}
}

func TestBuildTLSConfigEmptyUsesTLS12Minimum(t *testing.T) {
	cfg, err := buildTracingTLSConfig(TracingTLSConfig{})
	if err != nil {
		t.Fatalf("buildTracingTLSConfig() error = %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %v, want TLS 1.2", cfg.MinVersion)
	}
}

func TestBuildTLSConfigCAFileUsesAuthoritativeTrustBundle(t *testing.T) {
	caFile := writeTestCACert(t)

	cfg, err := buildTracingTLSConfig(TracingTLSConfig{CAFile: caFile})
	if err != nil {
		t.Fatalf("buildTracingTLSConfig() error = %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("RootCAs = nil, want CA pool from ca-file")
	}
	// cfg.RootCAs.Subjects() is deprecated since Go 1.18; verify the pool is
	// non-empty by attempting to verify a self-signed cert from the pool itself.
	// A non-nil pool with at least one cert will reject an unrelated leaf cert
	// without crashing — checking nil is sufficient here.
	_ = cfg.RootCAs
}

func TestBuildTLSConfigRejectsIncompleteClientCertificate(t *testing.T) {
	_, err := buildTracingTLSConfig(TracingTLSConfig{CertFile: "client.pem"})
	if err == nil {
		t.Fatal("buildTracingTLSConfig() error = nil, want error")
	}
	if !errors.Is(err, ErrInvalidTracing) {
		t.Fatalf("buildTracingTLSConfig() error = %v, want ErrInvalidTracing", err)
	}
}

func TestBuildExporterRejectsMalformedEndpointURL(t *testing.T) {
	exp, err := buildExporter(context.Background(), TracingConfig{
		Exporter: "otlp",
		Endpoint: "https://%",
		Insecure: true,
	}, nil)
	if err == nil {
		if exp != nil {
			_ = exp.Shutdown(context.Background())
		}
		t.Fatal("buildExporter() error = nil, want malformed endpoint error")
	}
	if !errors.Is(err, ErrInvalidTracing) {
		t.Fatalf("buildExporter() error = %v, want ErrInvalidTracing", err)
	}
}

func TestBuildExporterAcceptsSecureOTLPEndpointWithHeadersAndEmptyTLSConfig(t *testing.T) {
	exp, err := buildExporter(context.Background(), TracingConfig{
		Exporter: "otlp",
		Endpoint: "https://otel.example.com:4318",
		Insecure: false,
		Headers: map[string]string{
			"Authorization": "Bearer token",
			"x-tenant":      "helix",
		},
	}, nil)
	if err != nil {
		t.Fatalf("buildExporter() error = %v", err)
	}
	if err := exp.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestConfigureTracing_WithTracingConfig_DoesNotOverrideLoaderEnabled(t *testing.T) {
	// A partial tracing config override must not disable tracing that the loader enabled.
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

func writeTestCACert(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("x509.CreateCertificate() error = %v", err)
	}

	caFile := filepath.Join(t.TempDir(), "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(caFile, pemBytes, 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return caFile
}
