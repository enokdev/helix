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

## Complete command reference

See the [CLI Reference](/reference/cli) for a full list of every flag and option.
