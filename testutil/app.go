package testutil

import (
	"fmt"
	"sync"
	"testing"

	"github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
)

// App is a Helix application container scoped to a Go test.
type App struct {
	t         testing.TB
	container *core.Container
	config    config.Loader
	mu        sync.RWMutex
	closed    bool
}

// NewApp creates, configures, starts, and registers cleanup for a test app.
func NewApp(t testing.TB, options ...Option) *App {
	t.Helper()

	opts := defaultAppOptions()
	for _, option := range options {
		option(&opts)
	}

	loader := config.NewLoader(opts.configLoaderOptions()...)
	if err := loader.Load(new(struct{})); err != nil {
		t.Fatalf("testutil: load config: %v", err)
	}

	containerOptions := []core.Option{
		core.WithResolver(core.NewReflectResolver()),
		core.WithValueLookup(loader.Lookup),
	}
	containerOptions = append(containerOptions, opts.containerOptions...)
	container := core.NewContainer(containerOptions...)

	app := &App{
		t:         t,
		container: container,
		config:    loader,
	}

	for _, component := range opts.components {
		if err := container.Register(component); err != nil {
			t.Fatalf("testutil: register component %T: %v", component, err)
		}
	}

	if err := container.Start(); err != nil {
		if shutdownErr := container.Shutdown(); shutdownErr != nil {
			t.Fatalf("testutil: start: %v; shutdown after start failure: %v", err, shutdownErr)
		}
		t.Fatalf("testutil: start: %v", err)
	}

	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("testutil: cleanup: %v", err)
		}
	})

	return app
}

// Container returns the underlying Helix DI container.
func (a *App) Container() *core.Container {
	return a.container
}

// Config returns the configuration loader used by the test app.
func (a *App) Config() config.Loader {
	return a.config
}

// Close shuts down the test app once.
func (a *App) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true
	if err := a.container.Shutdown(); err != nil {
		return fmt.Errorf("testutil: close: %w", err)
	}
	return nil
}
