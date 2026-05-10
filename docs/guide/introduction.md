# Introduction

## What is Helix?

Helix is a **Go backend framework** built on top of [Fiber](https://gofiber.io/) that brings the ergonomics of Spring Boot to idiomatic Go. It gives you a production-grade application skeleton — dependency injection, convention-based routing, auto-configuration, security, observability, and scheduling — so you can focus on your domain logic instead of wiring infrastructure.

<CustomCallout type="tip" title="Philosophie">
  <strong>Convention over configuration, explicit over magic.</strong> Helix rend les patterns communs triviaux tout en gardant tout débogable et remplaçable.
</CustomCallout>

## Why Helix?

Building a Go REST API from scratch means assembling a router, a DI strategy, a config loader, middleware, structured logging, health checks, and database adapters — before writing a single line of business logic. Frameworks like Gin or Echo help with routing, but you still wire everything yourself.

Helix takes inspiration from Spring Boot's **auto-configuration** model: declare your components, describe your config, and the framework does the rest.

## Design Principles

| Principle | What it means |
|-----------|---------------|
| **Struct tags over annotations** | `inject:"true"` and `validate:"required"` — not Java-style reflection magic |
| **Compile-time over runtime** | Wire mode generates DI code at build time; reflection is an opt-in default |
| **Idiomatic Go** | No global state, no `init()` surprises, explicit dependency wiring |
| **Performance** | Startup under 100ms, low API latency via Fiber's fasthttp engine |
| **Testability** | First-class `NewTestApp` with real containers, not mocks-of-mocks |

## Feature Overview

### Dependency Injection

A type-safe DI container with automatic field injection via `inject:"true"` struct tags. Supports singleton and prototype scopes, cycle detection, and two resolution modes:

- **ReflectResolver** — runtime reflection, zero setup (default)
- **WireResolver** — compile-time code generation, zero runtime overhead

### Convention-Based Routing

Embed `helix.Controller` and name your methods after REST conventions:

| Method name | HTTP route |
|-------------|------------|
| `Index()` | `GET /resource` |
| `Show()` | `GET /resource/:id` |
| `Create()` | `POST /resource` |
| `Update()` | `PUT /resource/:id` |
| `Delete()` | `DELETE /resource/:id` |

Override with `//helix:route POST /auth/login` directives.

### Auto-Configuration Starters

Starters activate automatically based on your `go.mod` dependencies and config file:

- **Web** — Fiber HTTP server + lifecycle management
- **Data** — GORM database connection + repository pattern
- **Security** — JWT service + guard factories
- **Observability** — Prometheus, slog, OpenTelemetry, actuator endpoints
- **Scheduling** — Cron runner with graceful shutdown

### Configuration

Viper-backed loader with a clear priority chain:

```
ENV variables  >  application-{profile}.yaml  >  application.yaml  >  defaults
```

Profiles, dynamic reload on SIGHUP, and `mapstructure` struct binding included.

## Comparison

| Feature | Raw Fiber | Echo | Helix |
|---------|-----------|------|-------|
| HTTP routing | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#27c93f" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><polyline points="20 6 9 17 4 12"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#27c93f" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><polyline points="20 6 9 17 4 12"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#27c93f" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><polyline points="20 6 9 17 4 12"/></svg> Convention + custom |
| Dependency injection | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#27c93f" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><polyline points="20 6 9 17 4 12"/></svg> Built-in |
| Auto-configuration | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#27c93f" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><polyline points="20 6 9 17 4 12"/></svg> Starters |
| Config management | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#27c93f" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><polyline points="20 6 9 17 4 12"/></svg> YAML/ENV/profiles |
| JWT + RBAC | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#27c93f" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><polyline points="20 6 9 17 4 12"/></svg> Built-in |
| Health / metrics endpoints | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#27c93f" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><polyline points="20 6 9 17 4 12"/></svg> Actuators |
| Cron scheduling | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#27c93f" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><polyline points="20 6 9 17 4 12"/></svg> Built-in |
| Generic repository | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#27c93f" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><polyline points="20 6 9 17 4 12"/></svg> GORM adapter |
| TestApp utilities | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#ff5f56" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> | <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#27c93f" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="display: inline;"><polyline points="20 6 9 17 4 12"/></svg> Built-in |

## Package Layout

```
github.com/enokdev/helix
├── core/           # DI container, lifecycle management
├── web/            # Fiber HTTP integration, routing, middleware
├── data/           # Repository pattern, GORM adapter
├── config/         # YAML/ENV/TOML/JSON config loader
├── starter/        # Auto-configuration modules
├── observability/  # Prometheus, slog, OpenTelemetry, actuators
├── security/       # JWT, RBAC, SecurityConfigurer
├── scheduler/      # Cron job scheduling
└── cli/            # Project/module generator
```

## Mental model

Here is how a request flows through a Helix application:

```
helix.Run(App{Components: [...]})
         │
         ▼
  DI Container built
  (ReflectResolver resolves inject:"true" fields)
         │
         ▼
  Lifecycle.OnStart() called
  (database ping, cache warmup, scheduler start...)
         │
         ▼
  HTTP Server starts on :8080
         │
  ┌──────┴──────────────────────────────────────┐
  │  Incoming request                           │
  │                                             │
  │  Guard.CanActivate()  ──fail──▶  401/403   │
  │         │ pass                              │
  │         ▼                                  │
  │  Interceptor.Intercept()  (before)          │
  │         │                                  │
  │         ▼                                  │
  │  Handler(ctx, binding...) → (T, error)      │
  │         │                                  │
  │  Interceptor.Intercept()  (after)           │
  │         │                                  │
  │         ▼                                  │
  │  Error? → RequestError → JSON response     │
  │  T?     → JSON serialize → 200/201/204     │
  └──────────────────────────────────────────────┘
         │
  SIGTERM/SIGINT
         │
         ▼
  Lifecycle.OnStop() called in reverse order
```

## When to use Helix (and when not to)

**Use Helix when:**
- You're building a REST API or microservice in Go and want sensible defaults out of the box
- You want dependency injection without a separate wire-up file for every service
- Your team is familiar with Spring Boot and wants similar ergonomics in Go
- You need observability (metrics, health checks, tracing) without adding 10 libraries

**Consider alternatives when:**
- Your app is a CLI tool, a batch job, or a library — Helix is HTTP-server-centric
- You need maximum control over every HTTP detail — raw Fiber gives you more surface area
- Your team has already invested in a specific DI framework (e.g., `google/wire` alone)
- You're writing a single serverless function with no long-running state

## Migrating from Fiber / Echo / Gin

Helix runs Fiber under the hood, so existing Fiber middleware and handlers are compatible.

| Concept | Fiber / Echo / Gin | Helix |
|---------|--------------------|-------|
| Router setup | Manual `app.Get(...)` calls | Convention methods + `//helix:route` |
| Middleware | `app.Use(...)` | `RegisterInterceptor` + `//helix:interceptor` |
| Dependency injection | Manual constructor calls | `inject:"true"` struct tags |
| Config | `os.Getenv` / custom | `config.NewLoader` + YAML |
| Health check | Custom route | `/actuator/health` auto-registered |
| Auth guard | Custom middleware | `//helix:guard auth` |

A minimal migration path:

```go
// Before (raw Fiber):
app := fiber.New()
repo := &UserRepository{db: db}
svc := &UserService{repo: repo}
app.Get("/users", func(c *fiber.Ctx) error {
    return c.JSON(svc.List())
})
app.Listen(":8080")

// After (Helix):
type UserController struct {
    helix.Controller
    Svc *UserService `inject:"true"`
}
func (c *UserController) Index() []User { return c.Svc.List() }

helix.Run(helix.App{
    Components: []any{&UserRepository{db: db}, &UserService{}, &UserController{}},
})
```

## Next Steps

- [Installation](/guide/installation) — install Helix and the CLI
- [Quick Start](/guide/quick-start) — build your first API in 5 minutes
- [Dependency Injection](/guide/dependency-injection) — understand the DI container
- [Advanced DI](/guide/advanced-di) — Wire mode, interface injection, collect patterns
