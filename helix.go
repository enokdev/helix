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
	"sync"
	"syscall"
	"time"

	"github.com/enokdev/helix/config"
	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/security"
	"github.com/enokdev/helix/starter"
	starterdata "github.com/enokdev/helix/starter/data"
	starterobs "github.com/enokdev/helix/starter/observability"
	startersched "github.com/enokdev/helix/starter/scheduling"
	startersec "github.com/enokdev/helix/starter/security"
	starterweb "github.com/enokdev/helix/starter/web"
	"github.com/enokdev/helix/web"
)

var markerTypes = map[reflect.Type]struct{}{
	reflect.TypeOf(Service{}):            {},
	reflect.TypeOf(Controller{}):         {},
	reflect.TypeOf(Repository{}):         {},
	reflect.TypeOf(Component{}):          {},
	reflect.TypeOf(ErrorHandler{}):       {},
	reflect.TypeOf(SecurityConfigurer{}): {},
}

// RunMode selects the DI resolver strategy.
type RunMode string

const (
	// ModeReflect uses the default reflection-based resolver.
	ModeReflect RunMode = ""
	// ModeWire uses compile-time generated DI wiring.
	ModeWire RunMode = "wire"
)

var (
	wireSetupMu sync.Mutex
	wireSetupFn func(*core.Container) error

	webSetupMu sync.Mutex
	webSetupFn func() error
)

// App describes the application bootstrap configuration used by Run.
type App struct {
	// Scan lists package or filesystem patterns that should be inspected for
	// Helix component markers. Runtime scan cannot instantiate discovered Go
	// types by itself; provide Components for values that should be registered.
	Scan []string
	// Components contains already-instantiated components to auto-register.
	Components []any
	// Starters lists the auto-configuration modules to activate before
	// registering application components. Evaluated in canonical order.
	Starters []starter.Entry
	// ShutdownTimeout overrides the default lifecycle shutdown budget.
	ShutdownTimeout time.Duration
	// Logger overrides the logger used by lifecycle shutdown.
	Logger *slog.Logger
	// Mode selects the resolver strategy. The zero value keeps reflection mode.
	Mode RunMode

	awaitShutdown func() error
	// valueLookup is populated in zero-config mode to wire the auto-loaded
	// config into the container for value:"key" tag resolution.
	valueLookup func(string) (any, bool)
}

// ConfigReloadable is implemented by components that react after a successful
// configuration reload.
type ConfigReloadable = config.Reloadable

// Service marks a struct as a Helix service component.
type Service struct{}

// Controller marks a struct as a Helix controller component.
type Controller struct{}

// Repository marks a struct as a Helix repository component.
type Repository struct{}

// Component marks a struct as a generic Helix component.
type Component struct{}

// ErrorHandler marks a struct as a centralized HTTP error handler component.
type ErrorHandler struct{}

// SecurityConfigurer marks a struct as a global security configuration component.
type SecurityConfigurer struct{}

// RegisterWireSetup stores the bootstrap function emitted by generated wiring.
func RegisterWireSetup(fn func(*core.Container) error) {
	wireSetupMu.Lock()
	defer wireSetupMu.Unlock()
	wireSetupFn = fn
}

// RegisterWebSetup stores the bootstrap function emitted by generated web route and handler registrations.
func RegisterWebSetup(fn func() error) {
	webSetupMu.Lock()
	defer webSetupMu.Unlock()
	webSetupFn = fn
}

// Run builds the default reflection-based container, registers application
// components, starts lifecycle hooks, waits for shutdown, and stops cleanly.
//
// Run accepts an optional [App] argument. When called with no arguments (or
// with a zero-value [App]), the framework bootstraps automatically: it loads
// configuration from config/application.yaml or application.yaml (absence is
// not an error), auto-detects which starters to activate, and uses a
// registered wire setup function when available.
func Run(opts ...App) error {
	app := App{}
	if len(opts) > 0 {
		app = opts[0]
	}

	// Auto-bootstrap when no explicit configuration is provided.
	if isZeroApp(app) {
		return runAutoBootstrap(app)
	}

	return runWithApp(app)
}

// isZeroApp reports whether app carries no explicit configuration, meaning the
// caller wants fully automatic bootstrap.
func isZeroApp(app App) bool {
	return len(app.Components) == 0 &&
		len(app.Starters) == 0 &&
		len(app.Scan) == 0 &&
		app.Mode == ""
}

// autoLoadConfig creates a config loader that searches for application.yaml in
// the canonical paths but does not fail when no file is found.
func autoLoadConfig() config.Loader {
	return config.NewLoader(
		config.WithConfigPaths("config", "."),
		config.WithAllowMissingConfig(),
	)
}

// autoDetectStarters builds the list of starter entries whose conditions are
// satisfied given the current config loader. Starters that rely on
// component markers are included unconditionally; their MarkerAwareStarter
// condition is evaluated in the second pass inside runWithApp.
func autoDetectStarters(cfg config.Loader) []starter.Entry {
	entries := []starter.Entry{
		{
			Name:    "web",
			Order:   starter.OrderWeb,
			Starter: starterWebNew(cfg),
		},
		{
			Name:    "data",
			Order:   starter.OrderData,
			Starter: starterDataNew(cfg),
		},
		{
			Name:    "observability",
			Order:   starter.OrderObservability,
			Starter: starterObservabilityNew(cfg),
		},
		{
			Name:    "security",
			Order:   starter.OrderSecurity,
			Starter: starterSecurityNew(cfg),
		},
		{
			Name:    "scheduling",
			Order:   starter.OrderScheduling,
			Starter: starterSchedulingNew(cfg),
		},
	}
	return entries
}

// runAutoBootstrap handles the zero-config path: loads config automatically,
// auto-detects starters, and optionally uses a registered wire setup.
func runAutoBootstrap(base App) error {
	cfg := autoLoadConfig()

	// Trigger a load so that Lookup / AllSettings work during starter conditions.
	var settings map[string]any
	_ = cfg.Load(&settings)

	base.Starters = autoDetectStarters(cfg)
	// Wire the auto-loaded config into the container so that value:"key" tags
	// on application components are resolved from the config file.
	base.valueLookup = cfg.Lookup
	return runWithApp(base)
}

// starterWebNew creates the web starter backed by cfg.
func starterWebNew(cfg config.Loader) starter.Starter {
	return starterweb.New(cfg)
}

// starterDataNew creates the data starter backed by cfg.
func starterDataNew(cfg config.Loader) starter.Starter {
	return starterdata.New(cfg)
}

// starterObservabilityNew creates the observability starter backed by cfg.
func starterObservabilityNew(cfg config.Loader) starter.Starter {
	return starterobs.New(cfg)
}

// starterSecurityNew creates the security starter backed by cfg.
func starterSecurityNew(cfg config.Loader) starter.Starter {
	return startersec.New(cfg)
}

// starterSchedulingNew creates the scheduling starter backed by cfg.
func starterSchedulingNew(cfg config.Loader) starter.Starter {
	return startersched.New(cfg)
}

// runWithApp is the main implementation that executes a fully-populated App.
func runWithApp(app App) error {
	if app.Mode != ModeWire {
		if err := validateScan(app); err != nil {
			return err
		}
	}

	container := newDefaultContainer(app)

	starterLogger := app.Logger
	if starterLogger == nil {
		starterLogger = slog.Default()
	}

	// Pass 1: structural starters (config, web, data) that use go.mod or config
	// keys to determine activation. These must be configured before application
	// components are registered.
	if len(app.Starters) > 0 {
		if err := starter.Configure(container, app.Starters, starter.WithLogger(starterLogger)); err != nil {
			return fmt.Errorf("helix: configure starters: %w", err)
		}
	}

	if app.Mode == ModeWire {
		wireSetupMu.Lock()
		setup := wireSetupFn
		wireSetupMu.Unlock()
		if setup == nil {
			return fmt.Errorf("helix: wire setup not registered: %w", core.ErrUnresolvable)
		}
		if err := setup(container); err != nil {
			return fmt.Errorf("helix: wire setup: %w", err)
		}
	} else {
		// Auto-use wire setup when components list is empty and a setup is registered.
		if len(app.Components) == 0 {
			wireSetupMu.Lock()
			setup := wireSetupFn
			wireSetupMu.Unlock()
			if setup != nil {
				if err := setup(container); err != nil {
					return fmt.Errorf("helix: wire setup: %w", err)
				}
			}
			// No components and no wire setup: container stays empty; starters
			// provide their own components (web server, scheduler, etc.).
		} else if err := registerAppComponents(container, app.Components); err != nil {
			return err
		}
	}

	// Pass 2: cross-cutting starters (security, scheduling) that inspect the
	// container for component markers after application components are registered.
	if len(app.Starters) > 0 {
		if err := starter.ConfigureMarkerAware(container, app.Starters, starter.WithLogger(starterLogger)); err != nil {
			return fmt.Errorf("helix: configure marker-aware starters: %w", err)
		}
	}

	if err := autoRegisterControllers(container, app.Components); err != nil {
		return err
	}

	if err := applySecurityConfigurer(app, container); err != nil {
		return err
	}

	// Call generated web setup function if registered (registers pre-scanned routes and handlers)
	webSetupMu.Lock()
	webSetup := webSetupFn
	webSetupMu.Unlock()
	if webSetup != nil {
		if err := webSetup(); err != nil {
			return fmt.Errorf("helix: web setup: %w", err)
		}
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
	var resolver core.Resolver = core.NewReflectResolver()
	if app.Mode == ModeWire {
		resolver = core.NewWireResolver()
	}
	opts := []core.Option{core.WithResolver(resolver)}
	if app.ShutdownTimeout > 0 {
		opts = append(opts, core.WithShutdownTimeout(app.ShutdownTimeout))
	}
	if app.Logger != nil {
		opts = append(opts, core.WithLogger(app.Logger))
	}
	if app.valueLookup != nil {
		opts = append(opts, core.WithValueLookup(app.valueLookup))
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

func hasControllerMarker(component any) bool {
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

	controllerType := reflect.TypeOf(Controller{})
	for i := 0; i < componentType.NumField(); i++ {
		field := componentType.Field(i)
		if !field.Anonymous {
			continue
		}
		ft := field.Type
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		if ft == controllerType {
			return true
		}
	}
	return false
}

func awaitSignal() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	return nil
}

type securityConfigurer interface {
	Configure(hs *security.HTTPSecurity)
}

func applySecurityConfigurer(app App, container *core.Container) error {
	var configurer securityConfigurer
	for _, c := range app.Components {
		if cfg, ok := c.(securityConfigurer); ok {
			configurer = cfg
			break
		}
	}
	if configurer == nil {
		_ = container.Resolve(&configurer)
	}
	if configurer == nil {
		return nil
	}

	var server web.HTTPServer
	if err := container.Resolve(&server); err != nil {
		return fmt.Errorf("helix: apply security configurer: web server not registered: %w", err)
	}

	var jwtSvc security.JWTServicer
	var jSvc *security.JWTService
	if err := container.Resolve(&jSvc); err == nil {
		jwtSvc = jSvc
	}

	httpSec := security.NewHTTPSecurity(jwtSvc)
	configurer.Configure(httpSec)

	if err := web.ApplyGlobalGuard(server, httpSec.Build()); err != nil {
		return fmt.Errorf("helix: apply security configurer: %w", err)
	}

	return nil
}

// autoRegisterControllers discovers all controller components in the provided
// list and registers them with the HTTP server. It is called after
// registerAppComponents so that user-defined controllers are already in the
// container before registration takes place.
//
// If any controller uses a //helix:guard role directive, the "role" guard
// factory (security.NewRoleGuardFactory) is registered automatically before
// the controller routes are set up — idempotently, so a pre-registered factory
// is silently left in place.
func autoRegisterControllers(container *core.Container, components []any) error {
	if len(components) == 0 {
		return nil
	}

	var server web.HTTPServer
	if err := container.Resolve(&server); err != nil {
		// No HTTP server in the container — nothing to register.
		return nil
	}

	roleFactoryRegistered := false

	for _, component := range components {
		reg, isRegistration := component.(core.ComponentRegistration)
		var target any
		if isRegistration {
			target = reg.Component
		} else {
			target = component
		}

		if !hasControllerMarker(target) {
			continue
		}

		// Auto-register the "role" guard factory once, before registering the
		// first controller that might need it. The error is ignored because the
		// factory may already have been registered manually by the application.
		if !roleFactoryRegistered {
			_ = web.RegisterGuardFactory(server, "role", security.NewRoleGuardFactory())
			roleFactoryRegistered = true
		}

		controllerName := fmt.Sprintf("%T", target)
		if err := web.RegisterController(server, target); err != nil {
			return fmt.Errorf("helix: auto-register controller %s: %w", controllerName, err)
		}
	}
	return nil
}
