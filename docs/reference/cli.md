# CLI Reference

The Helix CLI scaffolds projects, generates code, manages database migrations, and runs your application during development.

## Installation

```bash
go install github.com/enokdev/helix/cmd/helix@latest
```

Verify:

```bash
helix version
```

::: details `helix: command not found` after install
`go install` places the binary in `$GOPATH/bin` (default `~/go/bin`). Add it to your `PATH`:

```bash
export PATH="$PATH:$HOME/go/bin"
source ~/.zshrc   # or ~/.bashrc
```
:::

---

## Typical dev session

```bash
# 1. Scaffold a new project
helix new app my-api
cd my-api

# 2. Fetch dependencies
go mod tidy

# 3. Generate a feature module
helix generate module order

# 4. Wire up components, write business logic…

# 5. Start with hot reload
helix run

# 6. Build a production binary
helix build
```

---

## `helix version`

Print the installed CLI version.

```bash
helix version
# or
helix --version
```

**Output:**

```
helix v1.1.2
```

---

## `helix new app`

Scaffold a new, ready-to-run Helix application.

```bash
helix new app <name> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `name` | Project name — becomes the directory name and Go module name |

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Parent directory in which to create the app folder |

**Examples:**

```bash
helix new app my-api
helix new app my-api --dir /workspace
```

**Generated structure:**

```
my-api/
├── main.go
├── go.mod
└── config/
    └── application.yaml
```

**`main.go`**

```go
package main

import (
    "log"

    "github.com/enokdev/helix"
)

func main() {
    if err := helix.Run(helix.App{}); err != nil {
        log.Fatal(err)
    }
}
```

**`config/application.yaml`**

```yaml
app:
  name: my-api
server:
  port: 8080
```

After scaffolding, run `go mod tidy` to download dependencies, then add feature modules with [`helix generate module`](#helix-generate-module).

---

## `helix generate module`

Add a feature module (controller + service + repository) to an existing project.

```bash
helix generate module <name> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `name` | Module name in singular form (e.g., `order`, `product`, `user`) |

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Go module root (must contain `go.mod`) |

**Examples:**

```bash
helix generate module order
helix generate module product --dir ./my-api
```

**Generated structure** (`helix generate module order`):

```
orders/
├── controller.go
├── service.go
├── repository.go
└── register.go
```

The folder name is automatically pluralized (`order` → `orders`). `register.go` calls `helix.RegisterComponents(...)` from `init()`, so generated components are wired automatically and do not need manual registration in `main.go`.

**`orders/repository.go`**

```go
package orders

import "github.com/enokdev/helix"

type OrderRepository struct {
    helix.Repository
}
```

**`orders/service.go`**

```go
package orders

import "github.com/enokdev/helix"

type OrderService struct {
    helix.Service
    Repository *OrderRepository `inject:"true"`
}
```

**`orders/controller.go`**

```go
package orders

import (
    "github.com/enokdev/helix"
    "github.com/enokdev/helix/web"
)

type OrderController struct {
    helix.Controller
    Service *OrderService `inject:"true"`
}

func (c *OrderController) Index(ctx web.Context) error {
    return ctx.JSON(map[string]string{"module": "orders"})
}
```

After generating, no `main.go` update is required. The generated `register.go` file handles DI registration automatically.

---

## `helix generate context`

Generate a DDD-style bounded context — a self-contained package with its own domain API, repository, service, and controller.

```bash
helix generate context <name> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `name` | Context name (e.g., `billing`, `inventory`, `accounts`) |

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Go module root (must contain `go.mod`) |

**Examples:**

```bash
helix generate context billing
helix generate context accounts --dir ./my-api
```

**Generated structure** (`helix generate context billing`):

```
billings/
├── api.go          # public domain functions (Create, Get)
├── repository.go   # data access
├── service.go      # business logic
├── controller.go   # HTTP layer
└── register.go     # auto-registration via init()
```

**`billings/api.go`** — the public entry point for the context, free of HTTP or DB concerns:

```go
package billings

import (
    "context"
    "errors"
)

var ErrNotImplemented = errors.New("billings: context operation not implemented")

type BillingID string

type Billing struct {
    ID BillingID
}

type CreateBillingAttrs struct {
    Name string
}

func CreateBilling(ctx context.Context, attrs CreateBillingAttrs) (*Billing, error) {
    return newBillingService().CreateBilling(ctx, attrs)
}

func GetBilling(ctx context.Context, id BillingID) (*Billing, error) {
    return newBillingService().GetBilling(ctx, id)
}
```

Use a bounded context when a feature has a clear domain boundary and you want to expose a clean Go API (not just HTTP routes) to the rest of the application. As with `helix generate module`, the generated `register.go` file registers the context components automatically, so `main.go` does not need manual component wiring.

---

## `helix generate` (wire code generation)

Scan the project and regenerate routing and DI wiring code from source annotations.

```bash
helix generate [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Directory tree to scan |

**What it generates:**

- Route registrations derived from controller method signatures
- Guard and interceptor registrations
- `helix_imports_gen.go` with blank imports for generated module/context packages so their `init()` auto-registrations run at startup

Run this after adding or renaming controllers, guards, interceptors, modules, or contexts.

```bash
helix generate
helix generate --dir ./my-api
```

---

## `helix generate wire`

Generate compile-time dependency injection bindings (Wire-style).

```bash
helix generate wire [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Directory tree to scan |

**When to use:** If you prefer compile-time DI over the reflection-based resolver. After running `helix generate wire`, the generated file replaces runtime `inject:"true"` resolution with explicit constructor calls.

```bash
helix generate wire
```

---

## `helix run`

Start the application with hot reload. Watches for source file changes and restarts automatically.

```bash
helix run [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Go module root |

**Examples:**

```bash
helix run
helix run --dir ./my-api
```

::: tip Dev vs production
`helix run` is for development only — it rebuilds and restarts on every save. For production, use `helix build` to produce a static binary and run that directly.
:::

Before each build, `helix run` executes `helix generate`, which refreshes `helix_imports_gen.go` so newly generated modules are auto-registered on startup.

The process handles `SIGINT` and `SIGTERM` gracefully: in-flight requests complete and lifecycle `OnStop()` hooks run before exit.

---

## `helix build`

Compile the application into a production binary (or generate a Dockerfile).

```bash
helix build [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Go module root |
| `--docker` | `false` | Generate a Dockerfile instead of building a binary |

**Examples:**

```bash
# Standard binary
helix build

# Dockerfile for container deployment
helix build --docker
```

The binary is output to the project root. The Docker flag produces a multi-stage `Dockerfile` with a minimal final image.

---

## `helix db migrate create`

Create a pair of timestamped migration SQL files.

```bash
helix db migrate create <name> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `name` | Descriptive migration name (use hyphens, e.g., `add-orders-table`) |

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Go module root |

**Example:**

```bash
helix db migrate create add-orders-table
```

**Generated files:**

```
migrations/
├── 20240115120000_add-orders-table.up.sql    # forward migration
└── 20240115120000_add-orders-table.down.sql  # rollback
```

Fill in the `.up.sql` with your schema change, and the `.down.sql` with the rollback:

```sql
-- 20240115120000_add-orders-table.up.sql
CREATE TABLE orders (
    id   TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    total REAL NOT NULL
);

-- 20240115120000_add-orders-table.down.sql
DROP TABLE IF EXISTS orders;
```

---

## `helix db migrate up`

Apply all pending migrations in chronological order.

```bash
helix db migrate up [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Go module root |
| `--database-url` | from config | Database connection URL (overrides `database.url` in `application.yaml`) |

**Examples:**

```bash
helix db migrate up
helix db migrate up --database-url postgres://localhost/mydb
```

---

## `helix db migrate down`

Roll back the most recently applied migration.

```bash
helix db migrate down [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Go module root |
| `--database-url` | from config | Database connection URL |

```bash
helix db migrate down
```

Each call rolls back exactly one migration. Run repeatedly to step back further.

---

## `helix db migrate status`

Show which migrations have been applied and which are pending.

```bash
helix db migrate status [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `.` | Go module root |
| `--database-url` | from config | Database connection URL |

**Example output:**

```
Migration                                     Status
----------------------------------------------------
20240115120000_create-users-table             applied
20240116090000_add-orders-table               applied
20240117140000_add-product-index              pending
```

---

## Command summary

| Command | Purpose |
|---------|---------|
| `helix version` | Print CLI version |
| `helix new app <name>` | Scaffold a new project |
| `helix generate module <name>` | Add a feature module (controller/service/repository) |
| `helix generate context <name>` | Add a DDD bounded context |
| `helix generate` | Regenerate routing and DI wiring code |
| `helix generate wire` | Generate compile-time DI bindings |
| `helix run` | Start with hot reload (development) |
| `helix build` | Compile production binary |
| `helix build --docker` | Generate Dockerfile |
| `helix db migrate create <name>` | Create migration files |
| `helix db migrate up` | Apply pending migrations |
| `helix db migrate down` | Roll back last migration |
| `helix db migrate status` | Show migration status |

---

## Environment variables

| Variable | Description |
|----------|-------------|
| `HELIX_PROFILES_ACTIVE` | Comma-separated list of active config profiles |
| `DATABASE_URL` | Overrides `database.url` in `application.yaml` |
| `SERVER_PORT` | Overrides `server.port` in `application.yaml` |
