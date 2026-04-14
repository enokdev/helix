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

## Commit Message Format

**ALWAYS** use conventional commits format. **NEVER** add a `Co-Authored-By` line or any other trailer.

Format: `<type>(<scope>): <description>`

Examples:
```
feat(core): implémenter le conteneur DI avec ReflectResolver
fix(web): corriger le routing par convention pour les méthodes DELETE
test(data): ajouter les tests d'intégration pour GormRepository
refactor(config): extraire la logique de priorité ENV > YAML > DEFAULT
```

Types: `feat`, `fix`, `test`, `refactor`, `docs`, `chore`, `perf`
Scopes: `core`, `web`, `data`, `config`, `starter`, `observability`, `scheduler`, `cli`

## GitHub Project Integration

**Repository:** `enokdev/helix` — **Project:** `https://github.com/orgs/enokdev/projects/1`

### When a story reaches `done` status

When a story is marked `done` (either during code-review or manually), **always** execute the following:

1. Find the corresponding GitHub issue by searching for the story number:
   ```bash
   gh issue list --repo enokdev/helix --search "Story <N>.<M>" --json number,title
   ```
2. Close the issue (this automatically closes the linked project task):
   ```bash
   gh issue close <number> --repo enokdev/helix --comment "Story implémentée et validée. ✅"
   ```

### Story → Issue mapping convention

GitHub issues were created with titles following the pattern: `Story N.M — <titre>`.  
Example: story key `1-3-reflectresolver-...` → search `"Story 1.3"` to find the issue number.
