# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Helix** is a Go backend framework inspired by Spring Boot, built on top of [Fiber](https://gofiber.io/). It aims to provide idiomatic Go equivalents of Spring Boot concepts: DI/IoC, auto-configuration, a repository pattern, observability endpoints, and a CLI tool. The project is currently in its **pre-code / planning phase** — only a PRD (`prd.md`) exists so far.

## Planned Architecture

```
helix/
 ├── core/           # DI container, lifecycle management, config
 ├── web/            # Fiber HTTP integration, routing, middleware
 ├── data/           # Repository pattern, ORM adapters (GORM/Ent/sqlc)
 ├── config/         # YAML/JSON/TOML/ENV loader (priority: ENV > YAML > DEFAULT)
 ├── starter/        # Auto-configuration modules (web, data, security, observability)
 ├── observability/  # Prometheus metrics, slog structured logging, OpenTelemetry
 └── cli/            # Project/module generator (`helix new app`, `helix generate module`)
```

## Core Design Principles

- **Struct tags over annotations**: use `inject:"true"` tags, not Java-style annotations
- **Compile-time over runtime**: prefer code generation (Wire-like) over reflection-based DI; reflection is an opt-in advanced mode
- **Idiomatic Go**: no magic, no global state, explicit dependency wiring
- **Performance targets**: startup < 100ms, low API latency

## Key Interfaces to Implement

**DI Container**
```go
container := helix.NewContainer()
container.Register(UserRepositoryImpl{})
container.Resolve(&UserService{})
```

**Repository generic interface**
```go
type Repository[T any] interface {
    FindAll() ([]T, error)
    FindByID(id int) (*T, error)
    Save(entity *T) error
    Delete(id int) error
}
```

**Lifecycle hooks**
```go
type Lifecycle interface {
    OnStart()
    OnStop()
}
```

**Observability endpoints**: `/health`, `/metrics` (Prometheus), `/info`

## Planned Starters

| Starter       | Responsibility              |
|---------------|-----------------------------|
| web           | Fiber auto-config           |
| data          | ORM + DB connection         |
| security      | JWT auth, RBAC middleware   |
| config        | YAML config loader          |
| observability | Prometheus + slog           |

## Development Setup (once code exists)

This will be a standard Go module project. Expected commands once initialized:

```bash
go mod tidy          # install dependencies
go build ./...       # build all packages
go test ./...        # run all tests
go test ./core/...   # run tests for a specific package
```

## Roadmap Phases

- **Phase 1 (MVP)**: DI container, YAML config loader, Fiber HTTP integration
- **Phase 2**: Repository pattern, auto-configuration, lifecycle management
- **Phase 3**: Observability, security module, CLI tool
- **Phase 4**: Cloud/microservices modules (Consul service discovery, circuit breaker), plugin system
