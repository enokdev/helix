# Installation

## Prerequisites

- **Go 1.21+** — Helix uses generics and the standard `log/slog` package
- **Git**
- **Node.js 18+** — only if you want to build this documentation locally

Verify your Go version:

```bash
go version
# go version go1.21.0 or later
```

## Install the CLI

The Helix CLI scaffolds new projects, generates code, and manages database migrations.

```bash
go install github.com/enokdev/helix/cmd/helix@latest
```

Verify:

```bash
helix --version
```

::: details `helix: command not found` after install

`go install` places the binary in `$GOPATH/bin` (default: `~/go/bin`). If that directory is not in your `PATH`, the shell can't find `helix`.

```bash
# Add to your ~/.zshrc or ~/.bashrc
export PATH="$PATH:$HOME/go/bin"

# Apply immediately
source ~/.zshrc
```

:::

::: details `compile: version "goX.Y.Z" does not match go tool version "goA.B.C"`

This happens when the `go` binary and `GOROOT` point to different Go installations — common when using a Go version manager (govm, gvm, asdf) alongside a system Go (Homebrew, golang.org installer).

Check your environment:

```bash
which go        # which binary is used
go env GOROOT   # must match that binary's installation
```

If they differ, unset `GOROOT` to let the Go binary use its own built-in path:

```bash
unset GOROOT
go install github.com/enokdev/helix/cmd/helix@latest
```

To fix permanently, remove any `export GOROOT=...` line from your shell config (`~/.zshrc`, `~/.bashrc`) that was added by the version manager, or ensure you use only one Go installation at a time.

:::

## Scaffold a New Project

```bash
helix new app my-api
cd my-api
```

This generates a ready-to-run project:

```
my-api/
├── main.go
├── go.mod
├── config/
│   └── application.yaml
└── internal/
    └── user/
        ├── controller.go
        ├── service.go
        └── repository.go
```

## Add to an Existing Project

```bash
go get github.com/enokdev/helix@latest
```

## Module Dependencies

Helix's core has minimal dependencies. Additional functionality is opt-in via starters:

| Capability | Extra dependency |
|------------|-----------------|
| HTTP server | `github.com/gofiber/fiber/v2` |
| SQLite database | `gorm.io/driver/sqlite` |
| PostgreSQL | `gorm.io/driver/postgres` |
| Prometheus metrics | `github.com/prometheus/client_golang` |
| OpenTelemetry | `go.opentelemetry.io/otel` |
| Cron scheduling | `github.com/robfig/cron/v3` |
| JWT security | `github.com/golang-jwt/jwt/v5` |

Starters detect these dependencies automatically — no manual registration needed.

## Editor Setup

Helix uses standard Go tooling. Any editor with `gopls` support works:

- **VS Code** — [Go extension](https://marketplace.visualstudio.com/items?itemName=golang.go)
- **GoLand** — built-in support
- **Neovim** — via `nvim-lspconfig` + `gopls`

## Next Steps

- [Quick Start](/guide/quick-start) — build and run your first API
