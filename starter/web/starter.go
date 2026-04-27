package web

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	helixconfig "github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/starter/internal/gomodutil"
	helixweb "github.com/enokdev/helix/web"
)

const (
	webStarterEnabledKey    = "helix.starters.web.enabled"
	serverPortKey           = "server.port"
	defaultServerPort       = "8080"
	shutdownTimeoutKey      = "helix.shutdown-timeout"
	defaultShutdownTimeout  = 30 * time.Second
)

// Starter auto-configures the HTTP server when Fiber is available.
type Starter struct {
	cfg helixconfig.Loader
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
func (s *Starter) Configure(container *core.Container) {
	if container == nil {
		return
	}

	port := defaultServerPort
	if s.cfg != nil {
		if value, ok := s.cfg.Lookup(serverPortKey); ok {
			if configuredPort := formatPort(value); configuredPort != "" {
				port = configuredPort
			}
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
	_ = container.Register(lifecycle)
	_ = container.Register(lifecycle.server)
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
	case int:
		return time.Duration(v)
	case int64:
		return time.Duration(v)
	case float64:
		return time.Duration(v)
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

func formatPort(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float32:
		return strconv.Itoa(int(v))
	case float64:
		return strconv.Itoa(int(v))
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	}
	return ""
}
