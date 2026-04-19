package web

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	helixconfig "github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
	helixweb "github.com/enokdev/helix/web"
)

const (
	webStarterEnabledKey = "helix.starters.web.enabled"
	serverPortKey        = "server.port"
	defaultServerPort    = "8080"
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
	data, err := os.ReadFile("go.mod")
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

	lifecycle := &serverLifecycle{
		server: helixweb.NewServer(),
		addr:   ":" + port,
	}
	_ = container.Register(lifecycle)
}

type serverLifecycle struct {
	server helixweb.HTTPServer
	addr   string
}

func (l *serverLifecycle) OnStart() error {
	if err := l.server.Start(l.addr); err != nil {
		return fmt.Errorf("web starter: start: %w", err)
	}
	return nil
}

func (l *serverLifecycle) OnStop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := l.server.Stop(ctx); err != nil {
		return fmt.Errorf("web starter: stop: %w", err)
	}
	return nil
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
