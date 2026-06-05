package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/starter"
	obsstarter "github.com/enokdev/helix/starter/observability"
	webstarter "github.com/enokdev/helix/starter/web"
	"github.com/enokdev/helix/web"
)

const healthCheckFlag = "--health-check"

type helloResponse struct {
	Message string `json:"message"`
}

type deploymentLoader struct {
	base config.Loader
}

func (l deploymentLoader) Load(target any) error {
	return l.base.Load(target)
}

func (l deploymentLoader) Lookup(key string) (any, bool) {
	switch key {
	case "server.port":
		if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
			return port, true
		}
		if value, ok := l.base.Lookup(key); ok {
			return value, true
		}
		return l.base.Lookup("helix.server.port")
	case "helix.logging.level":
		if level := strings.TrimSpace(os.Getenv("LOG_LEVEL")); level != "" {
			return level, true
		}
	}

	return l.base.Lookup(key)
}

func (l deploymentLoader) ConfigFileUsed() string {
	return l.base.ConfigFileUsed()
}

func (l deploymentLoader) AllSettings() map[string]any {
	return l.base.AllSettings()
}

func (l deploymentLoader) ActiveProfiles() []string {
	return l.base.ActiveProfiles()
}

func main() {
	loader, err := newLoader()
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) > 1 && os.Args[1] == healthCheckFlag {
		if err := runHealthCheck(loader); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := run(loader); err != nil {
		log.Fatal(err)
	}
}

func run(loader config.Loader) error {
	container := core.NewContainer(
		core.WithResolver(core.NewReflectResolver()),
		core.WithShutdownTimeout(10*time.Second),
	)

	entries := []starter.Entry{
		{Name: "web", Order: starter.OrderWeb, Starter: webstarter.New(loader)},
		{Name: "observability", Order: starter.OrderObservability, Starter: obsstarter.New(loader)},
	}
	if err := starter.Configure(container, entries); err != nil {
		return fmt.Errorf("deployment example: configure starters: %w", err)
	}

	var server web.HTTPServer
	if err := container.Resolve(&server); err != nil {
		return fmt.Errorf("deployment example: resolve http server: %w", err)
	}
	if err := server.RegisterRoute(http.MethodGet, "/api/hello", func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(helloResponse{Message: "hello from helix"})
	}); err != nil {
		return fmt.Errorf("deployment example: register hello route: %w", err)
	}

	if err := container.Start(); err != nil {
		return fmt.Errorf("deployment example: start: %w", err)
	}

	waitErr := waitForShutdown()
	shutdownErr := container.Shutdown()

	switch {
	case waitErr != nil && shutdownErr != nil:
		return errors.Join(
			fmt.Errorf("deployment example: wait for shutdown: %w", waitErr),
			fmt.Errorf("deployment example: shutdown: %w", shutdownErr),
		)
	case waitErr != nil:
		return fmt.Errorf("deployment example: wait for shutdown: %w", waitErr)
	case shutdownErr != nil:
		return fmt.Errorf("deployment example: shutdown: %w", shutdownErr)
	default:
		return nil
	}
}

func newLoader() (config.Loader, error) {
	base := config.NewLoader(
		config.WithConfigPaths("examples/deployment", "config", "."),
		config.WithAllowMissingConfig(),
		config.WithDefaults(map[string]any{
			"server.port":                          8080,
			"helix.server.port":                    8080,
			"helix.logging.level":                  "info",
			"helix.starters.web.enabled":           true,
			"helix.starters.observability.enabled": true,
		}),
	)

	var settings map[string]any
	if err := base.Load(&settings); err != nil {
		return nil, fmt.Errorf("deployment example: load config: %w", err)
	}

	return deploymentLoader{base: base}, nil
}

func runHealthCheck(loader config.Loader) error {
	port, err := resolvePort(loader)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://127.0.0.1:%s/actuator/health", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("deployment example: create health check request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("deployment example: health check request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("deployment example: health check status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

func resolvePort(loader config.Loader) (string, error) {
	if loader == nil {
		return "", errors.New("deployment example: config loader is nil")
	}

	value, ok := loader.Lookup("server.port")
	if !ok {
		return "8080", nil
	}

	port := strings.TrimSpace(fmt.Sprint(value))
	if port == "" {
		return "", errors.New("deployment example: server.port is empty")
	}

	return port, nil
}

func waitForShutdown() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	return nil
}
