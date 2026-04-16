// Package helix is the root package of the Helix framework - a Go backend framework
// inspired by Spring Boot.
//
// See [core] for the DI container, [web] for HTTP routing, [data] for the repository
// pattern, and [config] for configuration loading.
package helix

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	"github.com/enokdev/helix/core"
)

var markerTypes = map[reflect.Type]struct{}{
	reflect.TypeOf(Service{}):    {},
	reflect.TypeOf(Controller{}): {},
	reflect.TypeOf(Repository{}): {},
	reflect.TypeOf(Component{}):  {},
}

// App describes the application bootstrap configuration used by Run.
type App struct {
	// Scan lists package or filesystem patterns that should be inspected for
	// Helix component markers. Runtime scan cannot instantiate discovered Go
	// types by itself; provide Components for values that should be registered.
	Scan []string
	// Components contains already-instantiated components to auto-register.
	Components []any
	// ShutdownTimeout overrides the default lifecycle shutdown budget.
	ShutdownTimeout time.Duration
	// Logger overrides the logger used by lifecycle shutdown.
	Logger *slog.Logger

	awaitShutdown func() error
}

// Service marks a struct as a Helix service component.
type Service struct{}

// Controller marks a struct as a Helix controller component.
type Controller struct{}

// Repository marks a struct as a Helix repository component.
type Repository struct{}

// Component marks a struct as a generic Helix component.
type Component struct{}

// Run builds the default reflection-based container, registers application
// components, starts lifecycle hooks, waits for shutdown, and stops cleanly.
func Run(app App) error {
	if err := validateScan(app); err != nil {
		return err
	}

	container := newDefaultContainer(app)
	if err := registerAppComponents(container, app.Components); err != nil {
		return err
	}

	if err := container.Start(); err != nil {
		return fmt.Errorf("helix: start: %w", err)
	}

	wait := app.awaitShutdown
	if wait == nil {
		wait = awaitSignal
	}

	waitErr := wait()
	shutdownErr := container.Shutdown()

	switch {
	case waitErr != nil && shutdownErr != nil:
		return errors.Join(
			fmt.Errorf("helix: wait for shutdown: %w", waitErr),
			fmt.Errorf("helix: shutdown: %w", shutdownErr),
		)
	case waitErr != nil:
		return fmt.Errorf("helix: wait for shutdown: %w", waitErr)
	case shutdownErr != nil:
		return fmt.Errorf("helix: shutdown: %w", shutdownErr)
	default:
		return nil
	}
}

func newDefaultContainer(app App) *core.Container {
	opts := []core.Option{core.WithResolver(core.NewReflectResolver())}
	if app.ShutdownTimeout > 0 {
		opts = append(opts, core.WithShutdownTimeout(app.ShutdownTimeout))
	}
	if app.Logger != nil {
		opts = append(opts, core.WithLogger(app.Logger))
	}
	return core.NewContainer(opts...)
}

func validateScan(app App) error {
	if len(app.Scan) == 0 {
		return nil
	}

	result, err := scanComponentMarkers(app.Scan)
	if err != nil {
		return err
	}
	if len(app.Components) == 0 && result.ComponentCount > 0 {
		return fmt.Errorf("helix: scan found %d component marker(s) but cannot instantiate runtime values: %w", result.ComponentCount, ErrScanRequiresComponents)
	}
	return nil
}

func registerAppComponents(container *core.Container, components []any) error {
	for _, component := range components {
		if err := validateAppComponent(component); err != nil {
			return err
		}
		if err := container.Register(component); err != nil {
			return fmt.Errorf("helix: register component %T: %w", component, err)
		}
	}
	return nil
}

func validateAppComponent(component any) error {
	registration, ok := component.(core.ComponentRegistration)
	if !ok {
		if !hasComponentMarker(component) {
			return componentError(component)
		}
		return nil
	}

	if registration.Scope == core.ScopePrototype && registration.Lazy {
		return fmt.Errorf("helix: component %T cannot be both prototype and lazy: %w", registration.Component, ErrInvalidComponent)
	}
	if !hasComponentMarker(registration.Component) {
		return componentError(registration.Component)
	}
	return nil
}

// componentError returns a specific ErrInvalidComponent error distinguishing
// "not a pointer" from "no marker embed", so callers get an actionable message.
func componentError(component any) error {
	if component != nil {
		if v := reflect.ValueOf(component); v.IsValid() && v.Kind() != reflect.Ptr {
			return fmt.Errorf("helix: component %T must be a pointer to a struct: %w", component, ErrInvalidComponent)
		}
	}
	return fmt.Errorf("helix: component %T has no Helix marker embed: %w", component, ErrInvalidComponent)
}

func hasComponentMarker(component any) bool {
	if component == nil {
		return false
	}

	value := reflect.ValueOf(component)
	if !value.IsValid() || value.Kind() != reflect.Ptr || value.IsNil() {
		return false
	}

	componentType := value.Elem().Type()
	if componentType.Kind() != reflect.Struct {
		return false
	}

	for i := 0; i < componentType.NumField(); i++ {
		field := componentType.Field(i)
		if !field.Anonymous {
			continue
		}
		if isMarkerType(field.Type) {
			return true
		}
	}
	return false
}

func isMarkerType(fieldType reflect.Type) bool {
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}
	if fieldType.Kind() != reflect.Struct {
		return false
	}

	_, ok := markerTypes[fieldType]
	return ok
}

func awaitSignal() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	return nil
}
