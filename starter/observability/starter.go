package observability

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	helixconfig "github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
	helixobs "github.com/enokdev/helix/observability"
	"github.com/enokdev/helix/starter/internal/starterutil"
	helixweb "github.com/enokdev/helix/web"
)

const obsEnabledKey = "helix.starters.observability.enabled"

// Starter auto-configures the observability stack (logging, actuator routes, metrics, tracing).
type Starter struct {
	cfg helixconfig.Loader
}

// New creates a Starter using the provided configuration loader.
func New(cfg helixconfig.Loader) *Starter {
	return &Starter{cfg: cfg}
}

// Condition reports whether the observability starter should be activated.
func (s *Starter) Condition() bool {
	if s.cfg == nil {
		return false
	}

	if value, ok := s.cfg.Lookup(obsEnabledKey); ok {
		enabled, parsed := starterutil.ParseBool(value)
		if parsed {
			return enabled
		}
		// Non-parsable value: do not disable silently — fall through to auto-detect.
	}

	// Auto-detect: activate if any top-level "observability" key is present.
	all := s.cfg.AllSettings()
	_, ok := all["observability"]
	return ok
}

// Configure registers the observability components into the DI container.
func (s *Starter) Configure(container *core.Container) {
	if container == nil {
		return
	}

	// Configure structured logging (global slog default).
	if _, err := helixobs.ConfigureLogging(s.cfg); err != nil {
		slog.Default().Warn("observability starter: configure logging",
			slog.String("error", err.Error()))
	}

	// Configure OTel tracing (opt-in via config).
	_, shutdownFn, err := helixobs.ConfigureTracing(s.cfg)
	if err != nil {
		slog.Default().Warn("observability starter: configure tracing",
			slog.String("error", err.Error()))
		// Call shutdownFn if non-nil to release any partially-allocated resources.
		if shutdownFn != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = shutdownFn(ctx)
		}
		shutdownFn = nil
	}

	// Resolve the HTTP server registered by the web starter.
	var server helixweb.HTTPServer
	if err := container.Resolve(&server); err != nil {
		_ = container.Register(&observabilityLifecycle{
			startErr: fmt.Errorf("observability starter: resolve HTTPServer: %w", err),
			shutdown: shutdownFn,
		})
		return
	}

	// Build health checker from container-registered indicators (empty is fine).
	checker, err := helixobs.HealthCheckerFromContainer(container)
	if err != nil {
		slog.Default().Warn("observability starter: resolve health indicators",
			slog.String("error", err.Error()))
		// NewCompositeHealthChecker() with no args never fails; guard defensively.
		empty, err2 := helixobs.NewCompositeHealthChecker()
		if err2 != nil {
			slog.Default().Error("observability starter: create fallback health checker",
				slog.String("error", err2.Error()))
			_ = container.Register(&observabilityLifecycle{
				startErr: fmt.Errorf("observability starter: create fallback health checker: %w", err2),
				shutdown: shutdownFn,
			})
			return
		}
		checker = empty
	}

	// Build info provider.
	infoProvider := helixobs.NewInfoProvider(s.cfg)

	// Register actuator routes.
	if err := helixobs.RegisterActuatorRoutes(server, checker, infoProvider); err != nil {
		slog.Default().Warn("observability starter: register actuator routes",
			slog.String("error", err.Error()))
	}

	// Register /actuator/metrics Prometheus endpoint.
	if err := helixobs.RegisterMetricsRoute(server, helixobs.Registry()); err != nil {
		slog.Default().Warn("observability starter: register metrics route",
			slog.String("error", err.Error()))
	}

	_ = container.Register(&observabilityLifecycle{shutdown: shutdownFn})
}

// observabilityLifecycle handles the tracing shutdown on container stop.
type observabilityLifecycle struct {
	shutdown func(context.Context) error
	startErr error
}

func (l *observabilityLifecycle) OnStart() error {
	return l.startErr
}

func (l *observabilityLifecycle) OnStop() error {
	if l.shutdown == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := l.shutdown(ctx); err != nil {
		return fmt.Errorf("observability starter: shutdown: %w", err)
	}
	return nil
}

// ─── helpers ────────────────────────────────────────────────────────────────
