# Using the CLI

The Helix CLI is your primary tool for creating projects, generating code, and managing the development lifecycle. This guide walks through each workflow in the order you'll typically encounter them.

## Installing the CLI

```bash
go install github.com/enokdev/helix/cmd/helix@latest
helix version
```

The binary lands in `$GOPATH/bin` (usually `~/go/bin`). If your shell can't find it, add that directory to your `PATH`.

---

## Creating a project

```bash
helix new app my-api
cd my-api
go mod tidy
```

`helix new app` creates a minimal, working scaffold:

```
my-api/
├── main.go                  # application entry point
├── go.mod                   # Go module (references helix at the right version)
└── config/
    └── application.yaml     # server port, app name, and any custom keys
```

The generated `main.go` calls `helix.Run()` — the single entry point that boots the DI container, starts the HTTP server, and handles OS signals:

```go
func main() {
    if err := helix.Run(helix.App{}); err != nil {
        log.Fatal(err)
    }
}
```

`helix.App` accepts a `Components` slice for manually wired components. When you use generated modules or contexts, Helix registers them automatically for you.

---

## Adding feature modules

Once the project exists, generate a feature module with:

```bash
helix generate module order
```

This creates four files in an `orders/` package (singular → plural automatically):

```
orders/
├── controller.go   # HTTP handler wired to the service
├── service.go      # business logic, receives the repository via DI
├── repository.go   # data access layer, embeds helix.Repository
└── register.go     # auto-registration via init()
```

Each file has the right embedded type and `inject:"true"` tag already in place — you only need to implement the methods. `register.go` calls `helix.RegisterComponents(...)`, so there is no manual `main.go` registration step for generated modules.

Helix still reads the `inject:"true"` tags and wires `OrderRepository → OrderService → OrderController` automatically.

---

## DDD bounded contexts

For features with a clear domain boundary, use a bounded context instead of a bare module:

```bash
helix generate context billing
```

This generates a `billings/` package with five files:

```
billings/
├── api.go          # pure domain functions: CreateBilling(), GetBilling()
├── repository.go   # data access
├── service.go      # business logic with Create/Get method stubs
├── controller.go   # HTTP routes
└── register.go     # auto-registration via init()
```

The key addition is `api.go`. It exposes domain operations as plain Go functions, decoupled from HTTP and the database. Other packages in your application can call `billings.CreateBilling(ctx, attrs)` without knowing anything about the HTTP layer or the DI container. Like generated modules, bounded contexts also get a `register.go` file, so no manual `main.go` component registration is needed.

Use a bounded context when:
- A feature has its own domain language (entities, value objects)
- You want a stable internal API that other packages can depend on
- The feature might eventually become its own service

---

## Regenerating code

After changing or adding controllers, run:

```bash
helix generate
```

This scans the project and regenerates the route registration and DI wiring files, including `helix_imports_gen.go` with blank imports that trigger generated module `init()` registrations. You'll typically do this after:
- Adding a new method to a controller
- Renaming a controller
- Adding or removing a guard or interceptor
- Adding a new generated module or context

For compile-time DI wiring (Wire-style, no runtime reflection), use:

```bash
helix generate wire
```

---

## Development: hot reload

During development, use `helix run` instead of `go run`:

```bash
helix run
```

It watches your source files for changes. When you save a `.go` file, it recompiles and restarts the process automatically — no manual stop/start cycle. Before each build it runs `helix generate`, which refreshes `helix_imports_gen.go` so new generated modules are auto-registered.

The process handles `SIGINT` (`Ctrl+C`) and `SIGTERM` gracefully: active requests complete, and `OnStop()` lifecycle hooks run before the process exits.

---

## Building for production

```bash
# Compile a static binary
helix build

# Generate a Dockerfile (multi-stage, minimal final image)
helix build --docker
```

The binary is placed in the project root. Ship it to any Linux server or container and run it directly — no Go runtime required.

---

## Database migrations

Helix manages schema migrations with timestamped Go files. The workflow:

### 1. Create a migration file

```bash
helix db migrate create add-orders-table
```

One file is created in `db/migrations/`:

```
db/migrations/
└── 20240115120000_add_orders_table.go
```

Edit `Up(ctx, tx)` and `Down(ctx, tx)`:

```go
func Up(ctx context.Context, tx *sql.Tx) error {
    _, err := tx.ExecContext(ctx, "CREATE TABLE orders (id INTEGER PRIMARY KEY)")
    return err
}

func Down(ctx context.Context, tx *sql.Tx) error {
    _, err := tx.ExecContext(ctx, "DROP TABLE orders")
    return err
}
```

### 2. Apply migrations

```bash
helix db migrate up
```

All pending migrations are applied in chronological order. To target a specific database (overriding `application.yaml`):

```bash
helix db migrate up --database-url sqlite://./app.db
```

SQLite is the supported execution target today. Run migrations with `CGO_ENABLED=1` because the SQLite driver uses CGo. Migration files run inside an isolated temporary module, so they cannot import packages from the host application. `up` uses a database lock; locks older than 15 minutes are considered stale and cleared.

### 3. Check status

```bash
helix db migrate status
```

```
Migration                                     Status
----------------------------------------------------
20240115120000_create-users-table             applied
20240116090000_add-orders-table               pending
```

### 4. Roll back

```bash
helix db migrate down
```

Rolls back one migration at a time (the most recent applied one).

---

## Generated code

`helix generate` creates two kinds of files:

### Route registration (`_helix_routes_gen.go`)

```go
// Code generated by helix generate. DO NOT EDIT.
package main

import (
    helix "github.com/enokdev/helix"
    "github.com/enokdev/helix/web"
    "my-api/orders"
)

func init() {
    helix.RegisterWebSetup(func() error {
        // Routes are registered here from controller directives
        return nil
    })
}
```

### Wire DI (`wire_gen.go`)

`helix generate wire` generates compile-time DI wiring:

```go
// Code generated by helix generate wire. DO NOT EDIT.
package main

import (
    helix "github.com/enokdev/helix"
    "github.com/enokdev/helix/core"
    "my-api/orders"
    "my-api/user"
)

func init() {
    helix.RegisterWireSetup(func(c *core.Container) error {
        userRepo := &user.Repository{}
        userSvc := &user.Service{Repo: userRepo}
        userCtrl := &user.Controller{Svc: userSvc}

        c.Register(userRepo)
        c.Register(userSvc)
        c.Register(userCtrl)

        orderRepo := &orders.OrderRepository{}
        orderSvc := &orders.OrderService{Repo: orderRepo, UserSvc: userSvc}
        orderCtrl := &orders.OrderController{Svc: orderSvc}

        c.Register(orderRepo)
        c.Register(orderSvc)
        c.Register(orderCtrl)
        return nil
    })
}
```

With wire mode, no reflection happens at runtime. Use it when:
- Startup time is critical
- You want compile-time verification that all dependencies are satisfied
- You're deploying to resource-constrained environments

## Build flags

Pass Go build flags via `helix build`:

```bash
# Embed version information:
helix build --ldflags="-X main.version=$(git describe --tags) -X main.commit=$(git rev-parse --short HEAD)"

# Cross-compile for Linux:
GOOS=linux GOARCH=amd64 helix build

# Disable CGO for a fully static binary (required for distroless/scratch Docker images):
CGO_ENABLED=0 helix build

# All combined:
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 helix build \
  --ldflags="-s -w -X main.version=1.2.3"
```

## Dockerfile (generated by `helix build --docker`)

```dockerfile
# --- Build stage ---
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .

# --- Runtime stage ---
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /app/server .
COPY config/ ./config/
EXPOSE 8080
ENTRYPOINT ["/app/server"]
```

The distroless base image has no shell, no package manager, and no `apt` — significantly smaller attack surface than `alpine`.

## CI/CD integration

Add code generation and build to your pipeline:

```yaml
# .github/workflows/ci.yml
- name: Generate
  run: helix generate

- name: Build
  run: CGO_ENABLED=0 GOOS=linux GOARCH=amd64 helix build

- name: Test
  run: go test ./...
```

If `helix generate` produces output that differs from what's committed, the pipeline fails — this catches stale generated files:

```yaml
- name: Check generated files are up to date
  run: |
    helix generate
    git diff --exit-code   # fails if generated files changed
```

## Complete command reference

See the [CLI Reference](/reference/cli) for a full list of every flag and option.
