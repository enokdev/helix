# Helix — Copilot Instructions

## What Helix Is

A Go backend framework inspired by Spring Boot, built on [Fiber](https://gofiber.io/). It brings DI/IoC, auto-configuration, a repository pattern, observability endpoints, and a CLI to idiomatic Go — without sacrificing Go's performance or explicitness.

## Package Layout

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

- **Struct tags over annotations** — use `inject:"true"` for DI, `mapstructure:"key"` for config mapping
- **Compile-time over runtime** — code generation (Wire-like) is the opt-in production DI mode; reflection is the default development mode; the two modes are **mutually exclusive per module**
- **No global state** — explicit dependency wiring throughout
- **Config resolution order**: `ENV > YAML profile > application.yaml > DEFAULT`
- **Starter activation**: a starter activates only if its dependency is present in `go.mod` AND its minimum config key is provided

## Key Interfaces

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

## Go Commands

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

Every Helix application exposes under `/actuator/`:
- `GET /actuator/health` — app + dependency health
- `GET /actuator/metrics` — Prometheus metrics
- `GET /actuator/info` — version, build info, active config

## Development Milestones

| Milestone | Scope |
|-----------|-------|
| MVP | DI container, YAML config, Fiber HTTP — full CRUD API in < 30 min |
| Stable | Repository pattern, auto-configuration, lifecycle management |
| Production-ready | Observability, security (JWT/RBAC), CLI tooling |
| Ecosystem | Cloud modules (Consul, circuit breaker), Ent/sqlc adapters, plugin system |

## Import Rules

- `core/` and `config/` — **zero imports** of other Helix packages
- `web/internal/` — only package allowed to import `gofiber/fiber`
- `data/gorm/` — only package allowed to import `gorm.io/gorm`

## What Not to Do

- Do not reference internal tracking IDs, sprint names, or milestone numbers in Go source files or godoc comments
- Do not use `interface{}` — use generics
- Do not `panic()` in framework code (only at init-time with an explicit message)
- Do not import `gofiber/fiber` outside `web/internal/`
- Do not store `context.Context` in structs — always pass as parameter

# git commit message guidelines
- not Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>" in commit messages

### Closing completed work items after review

When a work item is marked `done` in the internal tracking system:

#### 1. Commit and push

```bash
git add <source files changed during review>
git commit -m "fix(<scope>): code review patches — story <N.M>"
git push origin <current-branch>
```

Never add `Co-Authored-By` trailers.

#### 2. Verify CI

Poll until the run completes (up to ~5 minutes, every 30 s):

```bash
gh run list --repo enokdev/helix --branch <current-branch> --limit 1 --json status,conclusion,databaseId
```

If it fails, get the logs and fix before continuing:

```bash
gh run view <run-id> --repo enokdev/helix --log-failed
```

Do not close the issue or move the project task until CI is green.

#### 3. Close the GitHub issue

```bash
gh issue list --repo enokdev/helix --search "Story <N>.<M>" --json number,title
gh issue close <number> --repo enokdev/helix --comment "Implemented and validated. ✅"
```

#### 4. Move the GitHub project task to Done

```bash
# Discover IDs
gh project list --owner enokdev --format json
gh project field-list 1 --owner enokdev --format json
gh project item-list 1 --owner enokdev --format json

# Move item
gh project item-edit \
  --id <item-node-id> \
  --field-id <status-field-id> \
  --project-id <project-node-id> \
  --single-select-option-id <done-option-id>
```

5. Verify that the CI passes , if not, investigate and fix before closing the issue.
