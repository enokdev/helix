# AGENTS.md

## Project State

Helix is an **active Go backend framework** under development. The module is initialized, the package structure is in place, and the `core/` DI container interfaces are implemented. Planning documents are written in **French** and live in `_bmad-output/`.

## Source of Truth

- **Architecture**: `_bmad-output/planning-artifacts/architecture.md` — all technical decisions
- **Feature Specifications**: `_bmad-output/planning-artifacts/epics.md` — full requirements breakdown
- **Development Progress**: `_bmad-output/implementation-artifacts/sprint-status.yaml` — current status
- **Implementation Specs**: `_bmad-output/implementation-artifacts/*.md` — detailed specs with acceptance criteria

When architecture docs conflict with `prd.md`, the architecture doc takes precedence (it was derived from the PRD and refined).

## Go Module & Tooling

- Module: `github.com/enokdev/helix`
- Minimum Go: **1.21** (required for `slog` and stable generics)
- Linting: `golangci-lint` with vet, staticcheck, errcheck, gofumpt, deadcode, revive
- Formatting: `gofumpt` (not just `gofmt`)
- Testing: `go test ./...` stdlib + `testify/assert` + `testify/mock`
- CI: GitHub Actions — lint, test, build on push/PR; goreleaser on `v*` tags

## Commands

```bash
go build ./...                        # build all packages
go test ./...                         # run all tests
go test ./core/...                    # single package
go test -run TestName ./core/...      # single test
golangci-lint run                     # lint (must pass before commit)
```

## Package Rules

Strict import hierarchy — violations will cause circular dependencies:
- `core/` — **zero imports** of other Helix packages
- `config/` — **zero imports** of other Helix packages
- `web/internal/` — only place allowed to import `gofiber/fiber`
- `data/gorm/` — only place allowed to import `gorm.io/gorm`
- Public interfaces go in package root files (e.g. `core/container.go`)
- Private implementations go in `internal/` subdirectories

Package naming: lowercase, singular, no underscores or dashes (`core`, `config`, `web`, `data`, `testutil`).

## DI Architecture

Two mutually exclusive resolver modes behind a shared `Resolver` interface:
- **ReflectResolver** (default) — runtime reflection, zero configuration required
- **WireResolver** (opt-in) — compile-time code generation, zero reflection in production

A module uses one mode, never both. The `Container` delegates to whichever `Resolver` is configured.

## Key Conventions

- DI via struct tags: `inject:"true"` — not constructor injection
- Config mapping via: `mapstructure:"key"` and `value:"key"`
- Component discovery via struct embeds: `helix.Service`, `helix.Controller`, `helix.Repository`, `helix.Component`
- Code generation directives: `//helix:route`, `//helix:transactional`, `//helix:scheduled`, `//helix:guard`
- Config priority: `ENV > YAML profile > application.yaml > DEFAULT`
- Observability endpoints under `/actuator/`: `/actuator/health`, `/actuator/metrics`, `/actuator/info`

## Code Quality Rules

- **No planning terminology in code or docs**: do not reference internal tracking IDs, sprint names, or milestone numbers in Go source files, godoc comments, README, or CONTRIBUTING. Those belong in `_bmad-output/` only.
- Tests must be co-located (`*_test.go` in the same package, never in a `test/` subfolder)
- Table-driven tests required when testing multiple cases
- Error wrapping: `fmt.Errorf("package: action: %w", err)`
- No `panic()` in framework code (only at init-time with an explicit message)
- No `interface{}` — use generics (Go 1.21+)

## Performance Targets

- Startup: < 100ms
- P99 latency `/actuator/health`: < 5ms
- Test coverage on `core/`: > 80%


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
