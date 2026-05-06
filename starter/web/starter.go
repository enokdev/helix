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
func (s *Starter) Condition() bool {
	goModPath, err := gomodutil.FindGoModPath()
	if err != nil {
		slog.Debug("web starter: go.mod not found", "error", err)
		return false
	}

	data, err := os.ReadFile(goModPath)
	if err != nil || !bytes.Contains(data, []byte("gofiber/fiber")) {
		return false
	}

	if s.cfg == nil {
		return true
	}

	value, ok := s.cfg.Lookup(webStarterEnabledKey)
	if !ok {
		return true
	}
	enabled, parsed := parseBool(value)
	return !parsed || enabled
}

// Configure registers the HTTP server lifecycle in the container.
func (s *Starter) Configure(container *core.Container) error {
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
		server:          helixweb.NewServer(),
		addr:            ":" + port,
		shutdownTimeout: shutdownTimeout,
	}
	if err := container.Register(lifecycle.server); err != nil {
		return fmt.Errorf("web starter: register server: %w", err)
	}
	if err := container.Register(lifecycle); err != nil {
		return fmt.Errorf("web starter: register lifecycle: %w", err)
	}
	s.configuredFor = container
	return nil
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

func (l *serverLifecycle) OnStop() error {
	timeout := l.shutdownTimeout
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := l.server.Stop(ctx); err != nil {
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
