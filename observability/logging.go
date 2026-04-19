package observability

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/enokdev/helix/config"
)

// ErrInvalidLogging is returned when logging configuration is invalid.
var ErrInvalidLogging = errors.New("observability: invalid logging")

// LoggingConfig holds the logging configuration resolved from helix.logging.*.
type LoggingConfig struct {
	// Level is the global log level: debug, info, warn, error.
	Level string
	// Levels maps a namespace to its specific level, e.g. {"web": "debug"}.
	Levels map[string]string
}

// LoggingOption configures ConfigureLogging.
type LoggingOption func(*loggingOptions) error

type loggingOptions struct {
	output    io.Writer
	namespace string
	cfg       *LoggingConfig
}

// WithLoggingOutput redirects log output (useful in tests).
func WithLoggingOutput(w io.Writer) LoggingOption {
	return func(o *loggingOptions) error {
		if w == nil {
			return fmt.Errorf("observability: configure logging: output: %w", ErrInvalidLogging)
		}
		o.output = w
		return nil
	}
}

// WithDefaultNamespace sets the namespace injected when no explicit namespace is provided.
func WithDefaultNamespace(ns string) LoggingOption {
	return func(o *loggingOptions) error {
		if ns == "" {
			return fmt.Errorf("observability: configure logging: namespace: %w", ErrInvalidLogging)
		}
		o.namespace = ns
		return nil
	}
}

// WithLoggingConfig provides an explicit LoggingConfig, bypassing loader lookup.
func WithLoggingConfig(cfg LoggingConfig) LoggingOption {
	return func(o *loggingOptions) error {
		if cfg.Levels != nil {
			levels := make(map[string]string, len(cfg.Levels))
			for k, v := range cfg.Levels {
				levels[k] = v
			}
			cfg.Levels = levels
		}
		o.cfg = &cfg
		return nil
	}
}

// ConfigureLogging builds a structured JSON logger from loader (or opts) and installs
// it as slog.Default(). Returns the configured logger or an error.
// Note: this function calls slog.SetDefault; it is not safe to call concurrently.
func ConfigureLogging(loader config.Loader, opts ...LoggingOption) (*slog.Logger, error) {
	options := &loggingOptions{
		output:    os.Stdout,
		namespace: "app",
	}

	for _, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("observability: configure logging: %w", ErrInvalidLogging)
		}
		if err := opt(options); err != nil {
			return nil, err
		}
	}

	var cfg LoggingConfig
	if options.cfg != nil {
		cfg = *options.cfg
	} else {
		cfg = resolveLoggingConfig(loader)
	}

	globalLevel, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("observability: configure logging: %w", err)
	}

	namespaceLevels := make(map[string]slog.Level, len(cfg.Levels))
	for ns, raw := range cfg.Levels {
		lvl, err := parseLevel(raw)
		if err != nil {
			return nil, fmt.Errorf("observability: configure logging: namespace %q: %w", ns, err)
		}
		namespaceLevels[ns] = lvl
	}

	var globalVar slog.LevelVar
	globalVar.Set(globalLevel)

	// minimum is the lowest level that should reach Enabled so that namespace-specific
	// debug loggers are not blocked before Handle is called.
	minimum := globalLevel
	for _, lvl := range namespaceLevels {
		if lvl < minimum {
			minimum = lvl
		}
	}

	jsonHandler := slog.NewJSONHandler(options.output, &slog.HandlerOptions{
		Level: minimum,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				return slog.Attr{Key: "timestamp", Value: a.Value}
			}
			return a
		},
	})

	h := &namespaceLevelHandler{
		inner:           jsonHandler,
		globalLevel:     &globalVar,
		namespaceLevels: namespaceLevels,
		defaultNS:       options.namespace,
	}

	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger, nil
}

// Logger returns a *slog.Logger pre-annotated with the given namespace.
// It reads from the current slog.Default() so it always reflects the Helix configuration.
func Logger(namespace string) *slog.Logger {
	return slog.Default().With(slog.String("namespace", namespace))
}

// ─── namespace-level handler ────────────────────────────────────────────────

type namespaceLevelHandler struct {
	inner           slog.Handler
	globalLevel     *slog.LevelVar
	namespaceLevels map[string]slog.Level
	defaultNS       string
	// preAttrs are attributes carried by Logger.With before any record is handled.
	preAttrs []slog.Attr
}

func (h *namespaceLevelHandler) Enabled(_ context.Context, level slog.Level) bool {
	// Let through anything that might pass for any configured namespace.
	minimum := h.globalLevel.Level()
	for _, lvl := range h.namespaceLevels {
		if lvl < minimum {
			minimum = lvl
		}
	}
	return level >= minimum
}

func (h *namespaceLevelHandler) Handle(ctx context.Context, r slog.Record) error {
	// Determine effective namespace from pre-attrs and record attrs.
	ns := h.resolveNamespace(r)

	// Apply the correct level threshold.
	threshold := h.globalLevel.Level()
	if lvl, ok := h.namespaceLevels[ns]; ok {
		threshold = lvl
	}
	if r.Level < threshold {
		return nil
	}

	// Build a new record that injects exactly one top-level "namespace" field
	// without duplicating it.
	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)

	// Carry over record attrs, skipping any existing "namespace" key.
	r.Attrs(func(a slog.Attr) bool {
		if a.Key != "namespace" {
			nr.AddAttrs(a)
		}
		return true
	})

	// Append pre-attrs (from With calls), again skipping "namespace".
	for _, a := range h.preAttrs {
		if a.Key != "namespace" {
			nr.AddAttrs(a)
		}
	}

	// Inject the resolved namespace at the end so it appears in the JSON output.
	nr.AddAttrs(slog.String("namespace", ns))

	return h.inner.Handle(ctx, nr)
}

func (h *namespaceLevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := h.clone()
	clone.preAttrs = append(clone.preAttrs, attrs...)
	return clone
}

func (h *namespaceLevelHandler) WithGroup(name string) slog.Handler {
	clone := h.clone()
	clone.inner = h.inner.WithGroup(name)
	return clone
}

func (h *namespaceLevelHandler) clone() *namespaceLevelHandler {
	levels := make(map[string]slog.Level, len(h.namespaceLevels))
	for k, v := range h.namespaceLevels {
		levels[k] = v
	}
	preAttrs := make([]slog.Attr, len(h.preAttrs))
	copy(preAttrs, h.preAttrs)
	return &namespaceLevelHandler{
		inner:           h.inner,
		globalLevel:     h.globalLevel,
		namespaceLevels: levels,
		defaultNS:       h.defaultNS,
		preAttrs:        preAttrs,
	}
}

// resolveNamespace finds the "namespace" value from pre-attrs or record attrs.
// When multiple "namespace" attrs are present in pre-attrs, the last one wins.
func (h *namespaceLevelHandler) resolveNamespace(r slog.Record) string {
	// Check pre-attrs in reverse so the last With("namespace",...) wins.
	for i := len(h.preAttrs) - 1; i >= 0; i-- {
		if h.preAttrs[i].Key == "namespace" {
			return h.preAttrs[i].Value.String()
		}
	}
	// Then check record attrs.
	ns := ""
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "namespace" {
			ns = a.Value.String()
			return false
		}
		return true
	})
	if ns != "" {
		return ns
	}
	return h.defaultNS
}

// ─── config resolution ───────────────────────────────────────────────────────

func resolveLoggingConfig(loader config.Loader) LoggingConfig {
	cfg := LoggingConfig{Level: "info"}
	if loader == nil {
		return cfg
	}

	if v, ok := loader.Lookup("helix.logging.level"); ok {
		if s, ok := v.(string); ok {
			cfg.Level = s
		}
	}

	all := loader.AllSettings()
	if helixRaw, ok := all["helix"]; ok {
		if helixMap, ok := helixRaw.(map[string]any); ok {
			if loggingRaw, ok := helixMap["logging"]; ok {
				if loggingMap, ok := loggingRaw.(map[string]any); ok {
					if levelsRaw, ok := loggingMap["levels"]; ok {
						if levelsMap, ok := levelsRaw.(map[string]any); ok {
							cfg.Levels = make(map[string]string, len(levelsMap))
							for k, v := range levelsMap {
								if s, ok := v.(string); ok {
									cfg.Levels[k] = s
								}
							}
						}
					}
				}
			}
		}
	}

	return cfg
}

// ─── level parsing ───────────────────────────────────────────────────────────

func parseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level %q: %w", raw, ErrInvalidLogging)
	}
}
