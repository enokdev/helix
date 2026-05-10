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

`helix.App` accepts a `Components` slice where you register your services, repositories, and controllers. You'll fill this in as you add modules.

---

## Adding feature modules

Once the project exists, generate a feature module with:

```bash
helix generate module order
```

This creates three files in an `orders/` package (singular → plural automatically):

```
orders/
├── controller.go   # HTTP handler wired to the service
├── service.go      # business logic, receives the repository via DI
└── repository.go   # data access layer, embeds helix.Repository
```

Each file has the right embedded type and `inject:"true"` tag already in place — you only need to implement the methods.

**Register the components** in `main.go`:

```go
import "my-api/orders"

helix.Run(helix.App{
    Components: []any{
        &orders.OrderRepository{},
        &orders.OrderService{},
        &orders.OrderController{},
    },
})
```

Helix reads the `inject:"true"` tags and wires `OrderRepository → OrderService → OrderController` automatically.

---

## DDD bounded contexts

For features with a clear domain boundary, use a bounded context instead of a bare module:

```bash
helix generate context billing
```

This generates a `billings/` package with four files:

```
billings/
├── api.go          # pure domain functions: CreateBilling(), GetBilling()
├── repository.go   # data access
├── service.go      # business logic with Create/Get method stubs
└── controller.go   # HTTP routes
```

The key addition is `api.go`. It exposes domain operations as plain Go functions, decoupled from HTTP and the database. Other packages in your application can call `billings.CreateBilling(ctx, attrs)` without knowing anything about the HTTP layer or the DI container.

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

This scans the project and regenerates the route registration and DI wiring files. You'll typically do this after:
- Adding a new method to a controller
- Renaming a controller
- Adding or removing a guard or interceptor

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

It watches your source files for changes. When you save a `.go` file, it recompiles and restarts the process automatically — no manual stop/start cycle.

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

Helix manages schema migrations with plain SQL files. The workflow:

### 1. Create migration files

```bash
helix db migrate create add-orders-table
```

Two files are created in `migrations/`:

```
migrations/
├── 20240115120000_add-orders-table.up.sql
└── 20240115120000_add-orders-table.down.sql
```

Edit them with your SQL:

```sql
-- up.sql
CREATE TABLE orders (
    id      TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    total   REAL NOT NULL
);

-- down.sql
DROP TABLE IF EXISTS orders;
```

### 2. Apply migrations

```bash
helix db migrate up
```

All pending migrations are applied in chronological order. To target a specific database (overriding `application.yaml`):

```bash
helix db migrate up --database-url postgres://user:pass@localhost/mydb
```

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
