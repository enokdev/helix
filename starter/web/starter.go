package web

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	helixconfig "github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/starter/internal/gomodutil"
	helixweb "github.com/enokdev/helix/web"
)

const (
	webStarterEnabledKey   = "helix.starters.web.enabled"
	serverPortKey          = "server.port"
	defaultServerPort      = "8080"
	shutdownTimeoutKey     = "helix.shutdown-timeout"
	defaultShutdownTimeout = 30 * time.Second
)

// ErrInvalidPort is returned when server.port is outside the valid TCP port range.
var ErrInvalidPort = errors.New("invalid port")

// Starter auto-configures the HTTP server when Fiber is available.
type Starter struct {
	cfg           helixconfig.Loader
	mu            sync.Mutex
	configuredFor *core.Container
}

// New creates a Starter using the provided configuration loader.
func New(cfg helixconfig.Loader) *Starter {
	return &Starter{cfg: cfg}
}

// Condition reports whether the web starter should be activated.
//
// Priority (highest to lowest):
//  1. helix.starters.web.enabled = false → inactive (absolute override)
//  2. go.mod not found in CWD tree       → helix.starters.web.enabled = true activates;
//     otherwise inactive (supports deployed binaries outside module root)
//  3. gofiber/fiber absent from go.mod   → inactive (dependency check, not bypassable)
//  4. helix.starters.web.enabled = true  → active
//  5. otherwise                          → active
func (s *Starter) Condition() bool {
	// Explicit disable is absolute regardless of go.mod state.
	if s.cfg != nil {
		if value, ok := s.cfg.Lookup(webStarterEnabledKey); ok {
			enabled, parsed := parseBool(value)
			if parsed && !enabled {
				return false
			}
		}
	}

	goModPath, err := gomodutil.FindGoModPath()
	if err != nil {
		// Binary launched from a directory without go.mod — common in production deployments.
		// Allow explicit enable to activate the starter in this case.
		if s.cfg != nil {
			if value, ok := s.cfg.Lookup(webStarterEnabledKey); ok {
				enabled, parsed := parseBool(value)
				if parsed && enabled {
					return true
				}
			}
		}
		slog.Debug("web starter: go.mod not found", "error", err)
		return false
	}

	// go.mod found — dependency must be present (enabled=true cannot bypass this check).
	data, err := os.ReadFile(goModPath)
	if err != nil || !bytes.Contains(data, []byte("gofiber/fiber")) {
		return false
	}

	// Dependency present — respect explicit enable/disable or default to true.
	if s.cfg != nil {
		if value, ok := s.cfg.Lookup(webStarterEnabledKey); ok {
			enabled, parsed := parseBool(value)
			if parsed {
				return enabled
			}
		}
	}
	return true
}

// Configure registers the HTTP server lifecycle in the container.
func (s *Starter) Configure(container *core.Container) error {
	return s.configure(container, slog.Default())
}

// ConfigureWithLogger registers the HTTP server using the application logger
// supplied by the starter orchestrator.
func (s *Starter) ConfigureWithLogger(container *core.Container, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	return s.configure(container, logger)
}

func (s *Starter) configure(container *core.Container, logger *slog.Logger) error {
	if container == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.configuredFor == container {
		return nil
	}

	port := defaultServerPort
	if s.cfg != nil {
		if value, ok := s.cfg.Lookup(serverPortKey); ok {
			configuredPort, err := parsePort(value)
			if err != nil {
				return err
			}
			port = configuredPort
		}
	}

	shutdownTimeout := defaultShutdownTimeout
	if s.cfg != nil {
		if value, ok := s.cfg.Lookup(shutdownTimeoutKey); ok {
			if d := parseDuration(value); d > 0 {
				shutdownTimeout = d
			}
		}
	}

	lifecycle := &serverLifecycle{
		server:          helixweb.NewServer(helixweb.WithLogger(logger)),
		addr:            ":" + port,
		shutdownTimeout: shutdownTimeout,
	}
	if err := container.Register(lifecycle.server); err != nil {
		return fmt.Errorf("web starter: register server: %w", err)
	}
	if err := container.Register(lifecycle); err != nil {
		registerErr := fmt.Errorf("web starter: register lifecycle: %w", err)
		if rollbackErr := rollbackRegistration(container, lifecycle.server); rollbackErr != nil {
			return errors.Join(registerErr, fmt.Errorf("web starter: rollback server: %w", rollbackErr))
		}
		return registerErr
	}
	s.configuredFor = container
	return nil
}

func rollbackRegistration(container *core.Container, component any) error {
	if container == nil || component == nil {
		return nil
	}
	return container.Unregister(component)
}

type serverLifecycle struct {
	server          helixweb.HTTPServer
	addr            string
	shutdownTimeout time.Duration
}

func (l *serverLifecycle) OnStart() error {
	if err := l.server.Start(l.addr); err != nil {
		return fmt.Errorf("web starter: start: %w", err)
	}
	return nil
}

func (l *serverLifecycle) OnStop(ctx context.Context) error {
	timeout := l.shutdownTimeout
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}
	stopCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := l.server.Stop(stopCtx); err != nil {
		return fmt.Errorf("web starter: stop: %w", err)
	}
	return nil
}

func parseDuration(value any) time.Duration {
	switch v := value.(type) {
	case time.Duration:
		return v
	case string:
		d, err := time.ParseDuration(strings.TrimSpace(v))
		if err != nil {
			return 0
		}
		return d
	}
	return 0
}

func parseBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes":
			return true, true
		case "false", "0", "no":
			return false, true
		}
	case int:
		return v != 0, true
	case int8:
		return v != 0, true
	case int16:
		return v != 0, true
	case int32:
		return v != 0, true
	case int64:
		return v != 0, true
	case uint:
		return v != 0, true
	case uint8:
		return v != 0, true
	case uint16:
		return v != 0, true
	case uint32:
		return v != 0, true
	case uint64:
		return v != 0, true
	}
	return false, false
}

func parsePort(value any) (string, error) {
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return "", invalidPort(value)
		}
		port, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return "", invalidPort(value)
		}
		return formatPortInt(port, value)
	case float32:
		return parseFloatPort(float64(v), value)
	case float64:
		return parseFloatPort(v, value)
	case int:
		return formatPortInt(int64(v), value)
	case int8:
		return formatPortInt(int64(v), value)
	case int16:
		return formatPortInt(int64(v), value)
	case int32:
		return formatPortInt(int64(v), value)
	case int64:
		return formatPortInt(v, value)
	case uint:
		return formatPortUint(uint64(v), value)
	case uint8:
		return formatPortUint(uint64(v), value)
	case uint16:
		return formatPortUint(uint64(v), value)
	case uint32:
		return formatPortUint(uint64(v), value)
	case uint64:
		return formatPortUint(v, value)
	}
	return "", invalidPort(value)
}

func parseFloatPort(port float64, original any) (string, error) {
	if math.IsNaN(port) || math.IsInf(port, 0) || math.Trunc(port) != port {
		return "", invalidPort(original)
	}
	return formatPortInt(int64(port), original)
}

func formatPortInt(port int64, original any) (string, error) {
	if port < 1 || port > 65535 {
		return "", invalidPort(original)
	}
	return strconv.FormatInt(port, 10), nil
}

func formatPortUint(port uint64, original any) (string, error) {
	if port < 1 || port > 65535 {
		return "", invalidPort(original)
	}
	return strconv.FormatUint(port, 10), nil
}

func invalidPort(value any) error {
	return fmt.Errorf("web starter: %s %v: %w", serverPortKey, value, ErrInvalidPort)
}
