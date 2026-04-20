package observability

import (
	"context"
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
		return otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(cfg.Endpoint),
			otlptracehttp.WithInsecure(),
		)
	default:
		return nil, fmt.Errorf("unsupported exporter %q: %w", cfg.Exporter, ErrInvalidTracing)
	}
}
