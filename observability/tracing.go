package observability

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/enokdev/helix/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingConfig holds the configuration for OpenTelemetry tracing.
type TracingConfig struct {
	Enabled     bool
	Exporter    string // "stdout" | "otlp" | "jaeger"
	Endpoint    string // OTLP HTTP endpoint, default "localhost:4318"
	ServiceName string // default "helix"
	Insecure    bool
	Headers     map[string]string
	TLS         TracingTLSConfig
}

// TracingTLSConfig holds optional OTLP HTTP TLS settings.
type TracingTLSConfig struct {
	CAFile     string
	CertFile   string
	KeyFile    string
	ServerName string
}

type tracingOptions struct {
	cfg    TracingConfig
	cfgSet bool
	output io.Writer
}

// TracingOption configures the tracing setup.
type TracingOption func(*tracingOptions)

// WithTracingConfig overrides the TracingConfig resolved from the loader.
// Intended for tests or manual wiring without a YAML file.
func WithTracingConfig(cfg TracingConfig) TracingOption {
	return func(o *tracingOptions) {
		o.cfg = cfg
		o.cfg.Headers = cloneTracingHeaders(cfg.Headers)
		o.cfgSet = true
	}
}

// WithTracingOutput sets the writer used by the stdout exporter.
// Defaults to os.Stdout when not provided.
// A nil writer is silently ignored.
func WithTracingOutput(w io.Writer) TracingOption {
	return func(o *tracingOptions) {
		if w == nil {
			return
		}
		o.output = w
	}
}

// ConfigureTracing initialises OpenTelemetry tracing from loader and opts.
//
// When tracing is disabled (default), ConfigureTracing returns (nil, nil, nil)
// and does NOT call otel.SetTracerProvider. The caller must check for a nil
// provider before passing it to web.WithTracerProvider.
//
// When tracing is enabled, ConfigureTracing calls otel.SetTracerProvider and
// otel.SetTextMapPropagator as a side effect. The returned shutdown function
// must be called (e.g. via defer) to flush and close the exporter.
//
// loader may be nil when WithTracingConfig is provided (test usage).
func ConfigureTracing(loader config.Loader, opts ...TracingOption) (trace.TracerProvider, func(context.Context) error, error) {
	o := &tracingOptions{
		output: os.Stdout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}

	cfg, err := resolveTracingConfig(loader, o)
	if err != nil {
		return nil, nil, fmt.Errorf("observability: configure tracing: %w", err)
	}

	if !cfg.Enabled {
		return nil, nil, nil
	}

	if err := validateExporter(cfg.Exporter); err != nil {
		return nil, nil, fmt.Errorf("observability: configure tracing: %w", err)
	}

	exp, err := buildExporter(context.Background(), cfg, o.output)
	if err != nil {
		return nil, nil, fmt.Errorf("observability: configure tracing: build exporter: %w", err)
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(semconv.ServiceNameKey.String(cfg.ServiceName)),
	)
	if err != nil && res == nil {
		// resource.New returns a partial resource on error; only fall back when
		// the partial resource itself is nil (which should not happen in practice).
		res = resource.NewSchemaless(semconv.ServiceNameKey.String(cfg.ServiceName))
	}

	var sp sdktrace.SpanProcessor
	if cfg.Exporter == "stdout" {
		sp = sdktrace.NewSimpleSpanProcessor(exp)
	} else {
		sp = sdktrace.NewBatchSpanProcessor(exp)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sp),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	shutdown := func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
	return tp, shutdown, nil
}

// resolveTracingConfig merges loader values and option overrides.
// Option overrides take priority over loader values.
func resolveTracingConfig(loader config.Loader, o *tracingOptions) (TracingConfig, error) {
	cfg := TracingConfig{
		Enabled:     false,
		Exporter:    "stdout",
		Endpoint:    "localhost:4318",
		ServiceName: "helix",
		Insecure:    true,
		Headers:     map[string]string{},
	}

	if loader != nil {
		if v, ok := loader.Lookup("helix.starters.observability.tracing.enabled"); ok {
			switch val := v.(type) {
			case bool:
				cfg.Enabled = val
			case string:
				cfg.Enabled = strings.EqualFold(strings.TrimSpace(val), "true")
			}
		}
		if v, ok := loader.Lookup("helix.starters.observability.tracing.exporter"); ok {
			if s, ok := v.(string); ok {
				// An explicit empty string is treated as an invalid value (AC9).
				cfg.Exporter = strings.TrimSpace(strings.ToLower(s))
			}
		}
		if v, ok := loader.Lookup("helix.starters.observability.tracing.endpoint"); ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				cfg.Endpoint = strings.TrimSpace(s)
			}
		}
		if v, ok := loader.Lookup("helix.starters.observability.tracing.service-name"); ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				cfg.ServiceName = strings.TrimSpace(s)
			}
		}
		if v, ok := loader.Lookup("helix.starters.observability.tracing.insecure"); ok {
			switch val := v.(type) {
			case bool:
				cfg.Insecure = val
			case string:
				cfg.Insecure = strings.EqualFold(strings.TrimSpace(val), "true")
			}
		}
		if v, ok := loader.Lookup("helix.starters.observability.tracing.headers"); ok {
			headers, err := resolveTracingHeaders(v)
			if err != nil {
				return cfg, err
			}
			cfg.Headers = headers
		}
		if v, ok := loader.Lookup("helix.starters.observability.tracing.tls.ca-file"); ok {
			if s, ok := v.(string); ok {
				cfg.TLS.CAFile = strings.TrimSpace(s)
			}
		}
		if v, ok := loader.Lookup("helix.starters.observability.tracing.tls.cert-file"); ok {
			if s, ok := v.(string); ok {
				cfg.TLS.CertFile = strings.TrimSpace(s)
			}
		}
		if v, ok := loader.Lookup("helix.starters.observability.tracing.tls.key-file"); ok {
			if s, ok := v.(string); ok {
				cfg.TLS.KeyFile = strings.TrimSpace(s)
			}
		}
		if v, ok := loader.Lookup("helix.starters.observability.tracing.tls.server-name"); ok {
			if s, ok := v.(string); ok {
				cfg.TLS.ServerName = strings.TrimSpace(s)
			}
		}
	}

	// WithTracingConfig overrides loader values.
	if o.cfgSet {
		if o.cfg.Exporter != "" {
			cfg.Exporter = strings.ToLower(strings.TrimSpace(o.cfg.Exporter))
		}
		if o.cfg.Endpoint != "" {
			cfg.Endpoint = o.cfg.Endpoint
		}
		if o.cfg.ServiceName != "" {
			cfg.ServiceName = o.cfg.ServiceName
		}
		if shouldOverrideTracingInsecure(loader, o.cfg) {
			cfg.Insecure = o.cfg.Insecure
		}
		if o.cfg.Headers != nil {
			cfg.Headers = cloneTracingHeaders(o.cfg.Headers)
		}
		if hasTracingTLSConfig(o.cfg.TLS) {
			cfg.TLS = o.cfg.TLS
		}
		// Apply Enabled from WithTracingConfig only when it is explicitly true,
		// or when no loader is active (test/manual path). This prevents
		// WithTracingConfig(TracingConfig{ServiceName: "x"}) from silently
		// disabling tracing that the loader had enabled.
		if o.cfg.Enabled || loader == nil {
			cfg.Enabled = o.cfg.Enabled
		}
	}

	return cfg, nil
}

func resolveTracingHeaders(v any) (map[string]string, error) {
	switch headers := v.(type) {
	case map[string]string:
		return cloneTracingHeaders(headers), nil
	case map[string]any:
		resolved := make(map[string]string, len(headers))
		for key, raw := range headers {
			value, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("header %q must be a string: %w", key, ErrInvalidTracing)
			}
			resolved[key] = value
		}
		return resolved, nil
	default:
		return nil, fmt.Errorf("headers must be map[string]string: %w", ErrInvalidTracing)
	}
}

func cloneTracingHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return nil
	}
	clone := make(map[string]string, len(headers))
	for key, value := range headers {
		clone[key] = value
	}
	return clone
}

func hasTracingTransportConfig(cfg TracingConfig) bool {
	return len(cfg.Headers) > 0 || hasTracingTLSConfig(cfg.TLS)
}

func shouldOverrideTracingInsecure(loader config.Loader, cfg TracingConfig) bool {
	return loader == nil || cfg.Insecure || cfg.Exporter != "" || cfg.Endpoint != "" || hasTracingTransportConfig(cfg)
}

func hasTracingTLSConfig(cfg TracingTLSConfig) bool {
	return cfg.CAFile != "" || cfg.CertFile != "" || cfg.KeyFile != "" || cfg.ServerName != ""
}

func validateExporter(exporter string) error {
	switch exporter {
	case "stdout", "otlp", "jaeger":
		return nil
	default:
		return fmt.Errorf("unsupported exporter %q: must be stdout, otlp, or jaeger: %w", exporter, ErrInvalidTracing)
	}
}

// buildExporter creates the SpanExporter for the given config.
// "jaeger" maps to OTLP HTTP pointed at the Jaeger OTLP port (default: localhost:4318).
// Jaeger must be configured with the OTLP receiver enabled
// (--collector.otlp.enabled=true or via the opentelemetry-collector config).
func buildExporter(ctx context.Context, cfg TracingConfig, output io.Writer) (sdktrace.SpanExporter, error) {
	switch cfg.Exporter {
	case "stdout":
		return stdouttrace.New(stdouttrace.WithWriter(output))
	case "otlp", "jaeger":
		opts := []otlptracehttp.Option{}
		if strings.HasPrefix(cfg.Endpoint, "http://") || strings.HasPrefix(cfg.Endpoint, "https://") {
			opts = append(opts, otlptracehttp.WithEndpointURL(cfg.Endpoint))
		} else {
			opts = append(opts, otlptracehttp.WithEndpoint(cfg.Endpoint))
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(cloneTracingHeaders(cfg.Headers)))
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		} else {
			tlsConfig, err := buildTracingTLSConfig(cfg.TLS)
			if err != nil {
				return nil, err
			}
			opts = append(opts, otlptracehttp.WithTLSClientConfig(tlsConfig))
		}
		return otlptracehttp.New(ctx, opts...)
	default:
		return nil, fmt.Errorf("unsupported exporter %q: %w", cfg.Exporter, ErrInvalidTracing)
	}
}

func buildTracingTLSConfig(cfg TracingTLSConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: cfg.ServerName,
	}

	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read tracing TLS CA file: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if ok := pool.AppendCertsFromPEM(pem); !ok {
			return nil, fmt.Errorf("parse tracing TLS CA file: %w", ErrInvalidTracing)
		}
		tlsConfig.RootCAs = pool
	}

	if (cfg.CertFile == "") != (cfg.KeyFile == "") {
		return nil, fmt.Errorf("tracing TLS cert-file and key-file must be configured together: %w", ErrInvalidTracing)
	}
	if cfg.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load tracing TLS client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
