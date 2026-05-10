# Dependency Injection

Helix includes a built-in DI container with automatic field injection, lifecycle management, and two resolution modes.

## The Container

```go
import (
    helix "github.com/enokdev/helix"
    "github.com/enokdev/helix/core"
)

container := core.NewContainer()
```

### Options

```go
container := core.NewContainer(
    core.WithResolver(core.NewReflectResolver()),   // default
    core.WithShutdownTimeout(30 * time.Second),     // graceful shutdown budget
    core.WithLogger(slog.Default()),
    core.WithValueLookup(func(key string) (any, bool) {
        // scalar values resolved via value:"key" tags
        return os.LookupEnv(key)
    }),
)
```

## Component Markers

Embed a marker struct to declare a component's role. Helix uses these to auto-register components to the appropriate subsystem (HTTP, DI, etc.).

```go
type UserService struct {
    helix.Service     // business logic layer
}

type UserController struct {
    helix.Controller  // HTTP handler — auto-registered to router
}

type UserRepository struct {
    helix.Repository  // data access layer
}

type CacheManager struct {
    helix.Component   // generic component
}
```

## Registering Components

```go
container.Register(&UserRepository{})
container.Register(&UserService{})
container.Register(&UserController{})
```

Registration order does not matter — Helix resolves the dependency graph automatically.

### Advanced Registration

Use `ComponentRegistration` for fine-grained control:

```go
container.Register(core.ComponentRegistration{
    Component:   &UserService{},
    Scope:       core.ScopePrototype,    // new instance per resolution
    Lazy:        true,                   // don't resolve until first use
    ResolveAs:   []reflect.Type{reflect.TypeOf((*UserReader)(nil)).Elem()},
    ExcludeFrom: []reflect.Type{reflect.TypeOf((*Auditable)(nil)).Elem()},
})
```

| Field | Description |
|-------|-------------|
| `Scope` | `ScopeSingleton` (default) or `ScopePrototype` |
| `Lazy` | Delay resolution until first use |
| `ResolveAs` | Limit which interface types this satisfies |
| `ExcludeFrom` | Remove from specific interface resolutions |

## Field Injection

Annotate struct fields with `inject:"true"`:

```go
type OrderService struct {
    helix.Service
    UserSvc *UserService       `inject:"true"` // resolved by type
    PaySvc  *PaymentService    `inject:"true"`
    Logger  *slog.Logger       `inject:"true"`
}
```

### Value Injection

Inject scalar values from a lookup function:

```go
type EmailService struct {
    helix.Service
    SMTPHost string `value:"smtp.host"`  // resolved via WithValueLookup
    SMTPPort int    `value:"smtp.port"`
}
```

## Resolving Components

```go
// Resolve a single component
var svc *UserService
if err := container.Resolve(&svc); err != nil {
    log.Fatal(err)
}

// Resolve all components implementing an interface
providers, err := core.ResolveAll[HealthIndicator](container)
```

## Resolution Modes

### Reflect Mode (default)

Uses Go reflection at runtime. Zero setup — suitable for most applications.

```go
container := core.NewContainer(
    core.WithResolver(core.NewReflectResolver()),
)
```

### Wire Mode (compile-time)

Generates DI wiring at build time via `helix generate`. Zero runtime overhead.

```go
container := core.NewContainer(
    core.WithResolver(core.NewWireResolver()),
)
```

Run code generation:

```bash
helix generate
```

## Lifecycle

Components implementing `core.Lifecycle` participate in the application lifecycle:

```go
type core.Lifecycle interface {
    OnStart() error
    OnStop(ctx context.Context) error
}
```

```go
type DatabaseConnection struct {
    helix.Component
    db *sql.DB
}

func (d *DatabaseConnection) OnStart() error {
    return d.db.Ping()
}

func (d *DatabaseConnection) OnStop(ctx context.Context) error {
    return d.db.Close()
}
```

**Startup order** — topological sort by dependency graph, preserving registration order for equal-level components.

**Shutdown order** — reverse of startup, with a shared deadline context from `WithShutdownTimeout`.

```go
// Start all registered lifecycle components
if err := container.Start(); err != nil {
    log.Fatal(err)
}

// Stop all (called automatically on SIGTERM/SIGINT when using helix.Run)
if err := container.Shutdown(); err != nil {
    log.Fatal(err)
}
```

## Dependency Graph

Inspect the resolved dependency graph programmatically:

```go
graph := container.Graph()
// graph.Nodes — list of component type names
// graph.Edges — map[string][]string of dependencies
```

## Error Handling

| Error | Cause |
|-------|-------|
| `core.ErrNotFound` | No component matches the target type |
| `core.ErrCyclicDep` | Circular dependency detected |
| `core.ErrUnresolvable` | Component cannot be constructed |
| `core.ErrShutdownTimeout` | OnStop exceeded the shutdown budget |

```go
if errors.Is(err, core.ErrCyclicDep) {
    var cyclic *core.CyclicDepError
    errors.As(err, &cyclic)
    fmt.Println("cycle path:", cyclic.Path)
}
```

## Slice injection with `collect`

Use `collect:""` to inject **all registered components** that implement an interface:

```go
type HealthIndicator interface {
    Check(ctx context.Context) ComponentHealth
}

// Multiple implementations registered:
container.Register(&DatabaseIndicator{})
container.Register(&CacheIndicator{})
container.Register(&ExternalAPIIndicator{})

// Collected into a slice automatically:
type HealthService struct {
    helix.Service
    Indicators []HealthIndicator `collect:""`
    // receives [*DatabaseIndicator, *CacheIndicator, *ExternalAPIIndicator]
}

func (s *HealthService) OverallStatus(ctx context.Context) string {
    for _, ind := range s.Indicators {
        if ind.Check(ctx).Status == StatusDown {
            return "DOWN"
        }
    }
    return "UP"
}
```

This is how Helix itself discovers health indicators, scheduled job providers, and event listeners.

## Lazy initialization

Add the `lazy:""` tag to delay resolution until first access:

```go
type AdminService struct {
    helix.Service
    // Only created when first accessed — not at container startup
    Reports *ReportGenerator `inject:"true" lazy:""`
}
```

Useful for expensive resources (report generators, large caches) that are only needed on certain code paths.

## Interface injection

Register a concrete type and inject by interface:

```go
type NotificationService interface {
    Notify(to, message string) error
}

type SlackNotifier struct {
    helix.Service
    webhookURL string
}

func (s *SlackNotifier) Notify(to, message string) error { ... }

// Register concrete:
container.Register(&SlackNotifier{webhookURL: "https://hooks.slack.com/..."})

// Inject by interface:
type AlertService struct {
    helix.Service
    Notifier NotificationService `inject:"true"` // resolved to *SlackNotifier
}
```

For multiple implementations of the same interface, see [Advanced DI](/guide/advanced-di).

## Using `helix.Run`

In most applications you won't touch the container directly — `helix.Run` manages it:

```go
helix.Run(helix.App{
    Components: []any{
        &UserRepository{},
        &UserService{},
        &UserController{},
    },
    ShutdownTimeout: 30 * time.Second,
})
```

`helix.Run` creates the container, registers all components, activates starters, starts the HTTP server, and handles SIGTERM/SIGINT gracefully.

## Common errors

| Error | Cause | Fix |
|-------|-------|-----|
| `core.ErrNotFound` | No component matches the target type | Register the missing component |
| `core.ErrCyclicDep` | A → B → A dependency cycle | Introduce an interface or restructure |
| `core.ErrUnresolvable` | Component found but cannot be constructed | Check field tags, nil pointers |
| `core.ErrShutdownTimeout` | An `OnStop` blocked past the budget | Increase `ShutdownTimeout` or fix blocking code |

### Diagnosing a cycle

```go
if errors.Is(err, core.ErrCyclicDep) {
    var cyclic *core.CyclicDepError
    errors.As(err, &cyclic)
    fmt.Println("cycle path:", strings.Join(cyclic.Path, " → "))
    // "cycle path: *UserService → *OrderService → *UserService"
}
```

For Wire mode, compile-time injection, and all advanced patterns, see [Advanced DI](/guide/advanced-di).
