# Helix — Copilot Instructions

## Project Status

Helix is currently in the **pre-code / planning phase**. Only `prd.md` (the product requirements document, written in French) exists as source of truth. No Go source files exist yet. All architecture described below is planned, not yet implemented.

## What Helix Is

A Go backend framework inspired by Spring Boot, built on [Fiber](https://gofiber.io/). It brings DI/IoC, auto-configuration, a repository pattern, observability endpoints, and a CLI to idiomatic Go — without sacrificing Go's performance or explicitness.

## Planned Package Layout

```
helix/
 ├── core/           # DI container, lifecycle management
 ├── web/            # Fiber HTTP integration, routing, middleware
 ├── data/           # Repository pattern, ORM adapters (GORM → Ent → sqlc)
 ├── config/         # YAML/JSON/TOML/ENV loader
 ├── starter/        # Auto-configuration modules
 ├── observability/  # Prometheus metrics, slog, OpenTelemetry
 └── cli/            # `helix new`, `helix generate`, `helix run`
```

## Core Design Principles

- **Struct tags over annotations** — use `inject:"true"` tags for DI, `mapstructure:"key"` for config mapping
- **Compile-time over runtime** — code generation (Wire-like) is the default DI mode; reflection is opt-in and the two modes are **mutually exclusive per module**
- **No global state** — explicit dependency wiring throughout
- **Config resolution order**: `ENV > YAML > DEFAULT`
- **Starter activation**: a starter is active only if its dependency is present in `go.mod` AND its minimum config key is provided

## Key Interfaces (as specified in the PRD)

```go
// DI container
container := helix.NewContainer()
container.Register(UserRepositoryImpl{})
container.Resolve(&UserService{})

// Repository — ID is parameterized (int, int64, string, uuid.UUID, or any comparable)
type Repository[T any, ID any] interface {
    FindAll() ([]T, error)
    FindByID(id ID) (*T, error)
    FindWhere(filter Filter) ([]T, error)
    Save(entity *T) error
    Delete(id ID) error
    Paginate(page, size int) (Page[T], error)
    WithTransaction(tx Transaction) Repository[T, ID]
}

// Lifecycle hooks (called in reverse dependency order on shutdown)
type Lifecycle interface {
    OnStart() error
    OnStop() error
}

// Config reload callback for singleton components
type ConfigReloadable interface {
    OnConfigReload()
}
```

## Go Commands (once code exists)

```bash
go mod tidy          # install/sync dependencies
go build ./...       # build all packages
go test ./...        # run all tests
go test ./core/...   # run tests for a single package
go test -run TestName ./core/...  # run a single test
go test -cover ./core/...         # coverage (target: >80% on core/)
```

Minimum Go version: **1.21** (required for `slog`).

## Observability Endpoints

Every Helix application exposes:
- `GET /health` — app + dependency health
- `GET /metrics` — Prometheus metrics
- `GET /info` — version, build info, active config

## Roadmap Phases

| Phase | Gate condition |
|-------|---------------|
| 1 — MVP | CRUD API running with DI, YAML config, Fiber |
| 2 — Structure | Project maintainable/extensible without touching `core/` |
| 3 — Production | Full observability, security (JWT/RBAC), CLI |
| 4 — Ecosystem | Cloud modules (Consul, circuit breaker), Ent/sqlc, plugins |

## Out of Scope

- gRPC / WebSocket (Phase 1–2)
- Frontend / SSR
- Custom ORM — Helix wraps GORM, Ent, or sqlc
- Multi-language support
