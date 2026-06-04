# Starters

Starters are auto-configuration modules that activate based on your `go.mod` dependencies, config file, or registered components. They eliminate boilerplate by wiring infrastructure automatically.

## How Starters Work

When `helix.Run` boots:

1. All starters evaluate their `Condition()` — does the required dependency exist? Is the config key set?
2. Starters that pass the condition call `Configure(*core.Container)` to register their components
3. "Marker-aware" starters additionally scan the container for specific component types

You can always override with an explicit config key.

## Web Starter

**Package:** `github.com/enokdev/helix/starter/web`

### Activation

| Trigger | Condition |
|---------|-----------|
| `go.mod` | `github.com/gofiber/fiber/v2` present |
| Config key | `helix.starters.web.enabled: true` |

Force off: `helix.starters.web.enabled: false`

### What it registers

| Component | Type | Description |
|-----------|------|-------------|
| `web.HTTPServer` | `web.HTTPServer` | Fiber HTTP server configured on `server.port` |
| `*serverLifecycle` | `core.Lifecycle` | Starts/stops the server in the lifecycle |

### Config keys consumed

| Key | Default | Description |
|-----|---------|-------------|
| `server.port` | `"8080"` | Listen port |
| `helix.shutdown-timeout` | `"30s"` | Drain timeout on shutdown |

---

## Data Starter

**Package:** `github.com/enokdev/helix/starter/data`

### Activation

| Trigger | Condition |
|---------|-----------|
| `go.mod` | `gorm.io/driver/sqlite` present |
| Config key | `database.url` is set |

Force off: `helix.starters.data.enabled: false`

### What it registers

| Component | Type | Description |
|-----------|------|-------------|
| `*gorm.DB` | `*gorm.DB` | GORM database instance |
| `*datagorm.TransactionManager` | `*datagorm.TransactionManager` | Transaction manager |
| `*datagorm.DB` | `*datagorm.DB` | Helix DB wrapper with lifecycle |
| `*databaseLifecycle` | `core.Lifecycle` | Pings DB on start, closes on stop |

### Config keys consumed

| Key | Default | Description |
|-----|---------|-------------|
| `database.url` | — | SQLite file path or connection string |
| `database.pool.max-open` | `0` | Max open connections |
| `database.pool.max-idle` | `0` | Max idle connections |
| `helix.starters.data.auto-migrate` | `false` | Run GORM AutoMigrate on start |

### Options

```go
import datastarter "github.com/enokdev/helix/starter/data"

datastarter.New(loader,
    datastarter.WithAutoMigrateModels(&User{}, &Order{}, &Product{}),
)
```

---

## Security Starter

**Package:** `github.com/enokdev/helix/starter/security`

### Activation

This is a **MarkerAwareStarter** — it checks the container *after* app components are registered.

| Trigger | Condition |
|---------|-----------|
| Config section | `security.*` keys present |
| Config key | `helix.starters.security.enabled: true` |
| Component marker | `helix.SecurityConfigurer` found in container |

Force off: `helix.starters.security.enabled: false`

### What it registers

| Component | Type | Description |
|-----------|------|-------------|
| `*security.JWTService` | `*security.JWTService` | JWT generation and validation |

### Config keys consumed

| Key | Default | Description |
|-----|---------|-------------|
| `security.jwt.secret` | — | JWT signing key (required) |
| `security.jwt.expiry` | `"24h"` | Token expiry duration |

---

## Observability Starter

**Package:** `github.com/enokdev/helix/starter/observability`

### Activation

| Trigger | Condition |
|---------|-----------|
| Config section | `observability.*` keys present |
| Config key | `helix.starters.observability.enabled: true` |

Force off: `helix.starters.observability.enabled: false`

### What it registers / configures

| Action | Description |
|--------|-------------|
| Structured logging | Sets the global `slog.Logger` from `logging.*` config |
| OTel tracing | Configures a tracer provider when `helix.starters.observability.tracing.enabled: true` |
| Health checker | Discovers all `HealthIndicator` components and builds a `CompositeHealthChecker` |
| Actuator routes | Registers `/actuator/health`, `/actuator/info`, `/actuator/metrics` |
| Prometheus observer | Installs `HTTPMetricsObserver` on the HTTP server |
| `*observabilityLifecycle` | `core.Lifecycle` — shuts down the OTel tracer provider cleanly |

### Config keys consumed

| Key | Default | Description |
|-----|---------|-------------|
| `logging.level` | `"info"` | Global log level |
| `logging.levels` | — | Per-namespace overrides |
| `helix.starters.observability.tracing.enabled` | `false` | Enable OTel tracing |
| `helix.starters.observability.tracing.service-name` | `""` | OTel service name |
| `helix.starters.observability.tracing.exporter` | `"stdout"` | `stdout` \| `otlp` \| `jaeger` |
| `helix.starters.observability.tracing.endpoint` | `""` | Exporter endpoint URL |
| `helix.starters.observability.tracing.insecure` | `true` | Use plaintext/insecure OTLP transport for backward compatibility |
| `helix.starters.observability.tracing.headers` | — | Static OTLP exporter headers, for example auth or tenant headers |
| `helix.starters.observability.tracing.tls.ca-file` | `""` | PEM CA bundle for OTLP TLS |
| `helix.starters.observability.tracing.tls.cert-file` | `""` | Client certificate for mTLS |
| `helix.starters.observability.tracing.tls.key-file` | `""` | Client certificate key for mTLS |
| `helix.starters.observability.tracing.tls.server-name` | `""` | TLS server name override |

---

## Scheduling Starter

**Package:** `github.com/enokdev/helix/starter/scheduling`

### Activation

This is a **MarkerAwareStarter**.

| Trigger | Condition |
|---------|-----------|
| `go.mod` | `github.com/robfig/cron/v3` present |
| Component marker | `scheduler.ScheduledJobProvider` found in container |

### What it registers

| Component | Type | Description |
|-----------|------|-------------|
| `*scheduler.Scheduler` | `*scheduler.Scheduler` | Cron runner |
| `*scheduledJobRegistrar` | `core.Lifecycle` | Discovers providers on start, stops cron on stop |

---

## Activation Priority

For any starter:

1. `helix.starters.<name>.enabled: false` → **always inactive**, regardless of other conditions
2. `helix.starters.<name>.enabled: true` → **always active**
3. Automatic detection (go.mod, config sections, component markers)

---

## Writing a Custom Starter

```go
type MyStarter struct {
    cfg config.Loader
}

func (s *MyStarter) Condition() bool {
    // Activate when config key is present
    _, ok := s.cfg.Lookup("my-feature.enabled")
    return ok
}

func (s *MyStarter) Configure(container *core.Container) error {
    // Register your components
    return container.Register(&MyFeatureService{})
}

// Register with helix.Run:
helix.Run(helix.App{
    Starters: []starter.Entry{
        {
            Name:    "my-feature",
            Order:   starter.OrderData + 1, // after data starter
            Starter: &MyStarter{cfg: loader},
        },
    },
})
```

### Execution Order

| Constant | Value | Starter |
|----------|-------|---------|
| `starter.OrderConfig` | 0 | Configuration (always first) |
| `starter.OrderWeb` | 1 | Web / HTTP server |
| `starter.OrderData` | 2 | Database / repositories |
| `starter.OrderObservability` | 3 | Metrics, logging, tracing |
| `starter.OrderSecurity` | 4 | JWT, guards |
| `starter.OrderScheduling` | 5 | Cron scheduler |
