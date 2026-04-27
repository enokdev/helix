# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Helix** is a Go backend framework inspired by Spring Boot, built on top of [Fiber](https://gofiber.io/). It provides idiomatic Go equivalents of Spring Boot concepts: DI/IoC, auto-configuration, a repository pattern, observability endpoints, and a CLI tool.

## Package Layout

```
helix/
 ├── core/           # DI container, lifecycle management
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

## Key Interfaces

**DI Container**
```go
container := helix.NewContainer()
container.Register(UserRepositoryImpl{})
container.Resolve(&UserService{})
```

**Repository generic interface**
```go
type Repository[T any, ID any] interface {
    FindAll() ([]T, error)
    FindByID(id ID) (*T, error)
    Save(entity *T) error
    Delete(id ID) error
}
```

**Lifecycle hooks**
```go
type Lifecycle interface {
    OnStart() error
    OnStop() error
}
```

**Observability endpoints**: `/actuator/health`, `/actuator/metrics`, `/actuator/info`

## Starters

| Starter       | Responsibility              |
|---------------|-----------------------------|
| web           | Fiber auto-config           |
| data          | ORM + DB connection         |
| security      | JWT auth, RBAC middleware   |
| config        | YAML config loader          |
| observability | Prometheus + slog           |

## Development Commands

```bash
go mod tidy          # sync dependencies
go build ./...       # build all packages
go test ./...        # run all tests
go test ./core/...   # run tests for a specific package
golangci-lint run    # lint (must pass before commit)
```

## Commit Message Format

**ALWAYS** use conventional commits format. **NEVER** add a `Co-Authored-By` line or any other trailer.

Format: `<type>(<scope>): <description>`

Examples:
```
feat(core): implement ReflectResolver with singleton scope
fix(web): correct convention routing for DELETE methods
test(data): add integration tests for GormRepository
refactor(config): extract ENV > YAML > DEFAULT priority logic
```

Types: `feat`, `fix`, `test`, `refactor`, `docs`, `chore`, `perf`
Scopes: `core`, `web`, `data`, `config`, `starter`, `observability`, `scheduler`, `cli`

## GitHub Project Integration

**Repository:** `enokdev/helix` — **Project:** `https://github.com/orgs/enokdev/projects/1`

### Closing completed work items

When a work item is marked `done` in the internal tracking system, close the corresponding GitHub issue:

1. Find the issue by searching for the feature number:
   ```bash
   gh issue list --repo enokdev/helix --search "Story <N>.<M>" --json number,title
   ```
2. Close it:
   ```bash
   gh issue close <number> --repo enokdev/helix --comment "Implemented and validated. ✅"
   ```

3. Verify that the CI passes , if not, investigate and fix before closing the issue.
