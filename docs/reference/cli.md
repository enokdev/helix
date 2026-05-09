# CLI Reference

The Helix CLI scaffolds projects, generates code, manages database migrations, and runs your application.

## Installation

```bash
go install github.com/enokdev/helix/cmd/helix@latest
```

---

## `helix new app`

Scaffold a new Helix application.

```bash
helix new app <name> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `name` | Project name and directory |

**Flags:**

| Flag | Description |
|------|-------------|
| `--dir string` | Target directory (default: `./<name>`) |

**Example:**

```bash
helix new app my-api
helix new app my-api --dir /workspace/my-api
```

**Generated structure:**

```
my-api/
├── main.go
├── go.mod
├── go.sum
├── config/
│   └── application.yaml
└── internal/
    └── user/
        ├── controller.go
        ├── service.go
        └── repository.go
```

---

## `helix generate`

Regenerate code from your source files. Run after adding or changing controllers, components, or directives.

```bash
helix generate [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--dir string` | Project directory (default: `.`) |
| `--output string` | Output file path for generated code |

**What it generates:**

- Route registrations from `//helix:route` directives
- Wire bindings for compile-time DI (when using Wire mode)
- Guard and interceptor registrations

**Example:**

```bash
helix generate
helix generate --dir ./my-api
```

---

## `helix generate module`

Add a new feature module to your project.

```bash
helix generate module <name> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `name` | Module name (e.g., `order`, `product`) |

**Flags:**

| Flag | Description |
|------|-------------|
| `--dir string` | Project directory (default: `.`) |

**Example:**

```bash
helix generate module order
helix generate module product --dir ./my-api
```

**Generated files:**

```
internal/order/
├── controller.go   # OrderController with CRUD stubs
├── service.go      # OrderService stub
└── repository.go   # OrderRepository stub
```

---

## `helix generate context`

Generate a bounded context (multiple modules with shared infrastructure).

```bash
helix generate context <name> [flags]
```

**Example:**

```bash
helix generate context billing
```

---

## `helix build`

Compile the application binary.

```bash
helix build [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--dir string` | Project directory (default: `.`) |
| `--docker` | Build a Docker image instead of a binary |

**Example:**

```bash
# Standard binary build
helix build

# Docker image build
helix build --docker
```

---

## `helix run`

Run the application with hot reload — the process restarts automatically when source files change.

```bash
helix run [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--dir string` | Project directory (default: `.`) |

**Example:**

```bash
helix run
helix run --dir ./my-api
```

::: tip
`helix run` is intended for development. Use `helix build` + binary for production.
:::

---

## `helix migrate create`

Create a new database migration file.

```bash
helix migrate create <name> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `name` | Migration name (e.g., `add-orders-table`) |

**Flags:**

| Flag | Description |
|------|-------------|
| `--dir string` | Project directory (default: `.`) |

**Example:**

```bash
helix migrate create add-orders-table
```

**Generated files:**

```
migrations/
├── 20240115_120000_add-orders-table.up.sql
└── 20240115_120000_add-orders-table.down.sql
```

---

## `helix migrate up`

Apply pending migrations.

```bash
helix migrate up [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--dir string` | Project directory (default: `.`) |
| `--database-url string` | Database connection URL (overrides config) |

**Example:**

```bash
helix migrate up
helix migrate up --database-url postgres://localhost/mydb
```

---

## `helix migrate down`

Roll back the last applied migration.

```bash
helix migrate down [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--dir string` | Project directory (default: `.`) |
| `--database-url string` | Database connection URL |

---

## `helix migrate status`

Show which migrations have been applied.

```bash
helix migrate status [flags]
```

**Example output:**

```
Migration                                Status
-----------------------------------------
20240115_120000_create-users-table       applied
20240116_090000_add-orders-table         applied
20240117_140000_add-product-index        pending
```

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `HELIX_PROFILES_ACTIVE` | Comma-separated list of active profiles |
| `DATABASE_URL` | Overrides `database.url` in config |
| `SERVER_PORT` | Overrides `server.port` in config |
