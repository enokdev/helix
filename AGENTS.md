# AGENTS.md

## Project State

Helix is in **pre-code / planning phase**. No Go source files exist yet. The PRD (`prd.md`) and all planning artifacts in `_bmad-output/` are written in **French**.

The next implementation step is Story 1.1 (project scaffold): `_bmad-output/implementation-artifacts/1-1-initialisation-du-projet-structure-de-base.md`

## Source of Truth

- **PRD**: `prd.md` (French) — product requirements
- **Architecture**: `_bmad-output/planning-artifacts/architecture.md` — all technical decisions
- **Epics & Stories**: `_bmad-output/planning-artifacts/epics.md` — full backlog
- **Sprint Status**: `_bmad-output/implementation-artifacts/sprint-status.yaml` — current progress
- **Story Specs**: `_bmad-output/implementation-artifacts/*.md` — detailed story files with acceptance criteria

When architecture docs conflict with `prd.md`, the architecture doc takes precedence (it was derived from the PRD and refined).

## Go Module & Tooling

- Module: `github.com/enokdev/helix`
- Minimum Go: **1.21** (required for `slog` and stable generics)
- Linting: `golangci-lint` with vet, staticcheck, errcheck, gofumpt
- Formatting: `gofumpt` (not just `gofmt`)
- Testing: `go test ./...` + `testify/assert` + `testify/mock`
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
- Public interfaces go in package root files (e.g. `core/container.go`)
- Private implementations go in `internal/` subdirectories

Package naming: lowercase, singular, no underscores or dashes (`core`, `config`, `web`, `data`, `testutil`).

## DI Architecture

Two mutually exclusive resolver modes behind a shared `Resolver` interface:
- **ReflectResolver** (default) — runtime reflection, used for development
- **WireResolver** (opt-in) — compile-time codegen, used for production

A module uses one mode, never both. The `Container` delegates to whichever `Resolver` is configured.

## Key Conventions

- DI via struct tags: `inject:"true"` — not constructor injection
- Config mapping via: `mapstructure:"key"` and `value:"key"`
- Component discovery via struct embeds: `helix.Service`, `helix.Controller`, `helix.Repository`, `helix.Component`
- Code generation directives: `//helix:route`, `//helix:transactional`, `//helix:scheduled`, `//helix:guard`
- Config priority: `ENV > YAML profile > application.yaml > DEFAULT`
- Observability endpoints under `/actuator/` (not root): `/actuator/health`, `/actuator/metrics`, `/actuator/info`

## BMad Methodology

This repo uses BMad for planning. Skills are installed in `.opencode/skills/`, `.agent/skills/`, `.github/skills/`, `.gemini/skills/`, `.agents/skills/`, and `.claude/skills/`. Planning output lives in `_bmad-output/`. Do not modify BMad skill files — they are tooling, not project code.

## Performance Targets

- Startup: < 100ms
- P99 latency `/actuator/health`: < 5ms
- Test coverage on `core/`: > 80%
