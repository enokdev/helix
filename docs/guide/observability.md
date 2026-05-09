# Observability

Helix includes built-in structured logging, Prometheus metrics, OpenTelemetry tracing, and Spring-style actuator endpoints — all enabled via the observability starter.

## Actuator Endpoints

When the observability starter is active, three endpoints are registered automatically:

| Endpoint | Description |
|----------|-------------|
| `GET /actuator/health` | Application health — 200 UP, 503 DOWN |
| `GET /actuator/metrics` | Prometheus metrics in text format |
| `GET /actuator/info` | Application metadata (version, profiles, build info) |

Enable the starter via config:

```yaml
# config/application.yaml
observability:
  enabled: true   # presence of this section is enough to trigger auto-config
```

Or set explicitly:

```yaml
helix:
  starters:
    observability:
      enabled: true
```

## Health Checks

### Custom health indicators

```go
import "github.com/enokdev/helix/observability"

type DatabaseHealthIndicator struct {
    db *sql.DB
}

func (h *DatabaseHealthIndicator) Name() string {
    return "database"
}

func (h *DatabaseHealthIndicator) Health(ctx context.Context) observability.ComponentHealth {
    if err := h.db.PingContext(ctx); err != nil {
        return observability.ComponentHealth{
            Status: observability.StatusDown,
            Error:  err.Error(),
        }
    }
    return observability.ComponentHealth{
        Status: observability.StatusUp,
        Details: map[string]any{
            "driver": "postgres",
        },
    }
}
```

Register the indicator as a component — the observability starter discovers it automatically:

```go
helix.Run(helix.App{
    Components: []any{
        &DatabaseHealthIndicator{db: db},
        // ...
    },
})
```

### Health response format

```json
// GET /actuator/health  →  200 OK
{
  "status": "UP",
  "components": {
    "database": {
      "status": "UP",
      "details": { "driver": "postgres" }
    },
    "redis": {
      "status": "DOWN",
      "error": "connection refused"
    }
  }
}
```

Overall status is `DOWN` (HTTP 503) if **any** component is `DOWN`.

### Manual health checker

```go
checker, err := observability.NewCompositeHealthChecker(
    &DatabaseHealthIndicator{db: db},
    &CacheHealthIndicator{client: redis},
)

resp := checker.Check(ctx)
fmt.Println(resp.Status) // "UP" or "DOWN"
```

## Structured Logging

Helix uses Go's standard `log/slog` package with JSON output by default.

### Configure logging

```go
import "github.com/enokdev/helix/observability"

logger, err := observability.ConfigureLogging(loader,
    observability.WithLoggingOutput(os.Stdout),
    observability.WithDefaultNamespace("my-api"),
)
```

### YAML configuration

```yaml
logging:
  level: info          # global level: debug | info | warn | error
  levels:
    "my-api/data":   debug   # per-namespace level overrides
    "my-api/web":    warn
```

### Namespaced loggers

```go
logger := observability.Logger("my-api/user")
logger.Info("user created", "id", user.ID, "email", user.Email)
// {"level":"INFO","namespace":"my-api/user","msg":"user created","id":42,"email":"alice@example.com"}
```

### Inject the logger

```go
type UserService struct {
    helix.Service
    Log *slog.Logger `inject:"true"`
}
```

## Prometheus Metrics

### Auto-registered metrics

| Metric | Type | Labels |
|--------|------|--------|
| `helix_http_requests_total` | Counter | `method`, `route`, `status` |
| `helix_http_request_duration_seconds` | Histogram | `method`, `route`, `status` |

### Manual metrics

```go
import (
    "github.com/enokdev/helix/observability"
    "github.com/prometheus/client_golang/prometheus"
)

registry := observability.Registry()  // shared singleton

ordersTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "orders_total",
    Help: "Total number of orders processed",
}, []string{"status"})

registry.MustRegister(ordersTotal)

// Later:
ordersTotal.With(prometheus.Labels{"status": "success"}).Inc()
```

### Protected metrics endpoint

```go
import "github.com/enokdev/helix/observability"

observability.RegisterMetricsRoute(server, registry,
    observability.WithMetricsGuard(&InternalNetworkGuard{}),
)
```

### Metrics observer

```go
observer, err := observability.NewHTTPMetricsObserver(registry)

server := web.NewServer(
    web.WithRouteObserver(observer),
)
```

## OpenTelemetry Tracing

### Configure tracing

```go
import "github.com/enokdev/helix/observability"

tp, shutdown, err := observability.ConfigureTracing(loader,
    observability.WithTracingConfig(observability.TracingConfig{
        Enabled:     true,
        ServiceName: "my-api",
        Exporter:    "otlp",
        Endpoint:    "http://jaeger:4318",
    }),
)
if err != nil {
    log.Fatal(err)
}
defer shutdown(ctx)

// Pass to HTTP server for automatic span creation:
server := web.NewServer(
    web.WithTracerProvider(tp),
)
```

### YAML configuration

```yaml
observability:
  tracing:
    enabled: true
    service-name: "my-api"
    exporter: otlp        # otlp | stdout | jaeger
    endpoint: "http://jaeger:4318"
```

### Supported exporters

| Exporter | Use case |
|----------|----------|
| `stdout` | Development / debugging |
| `otlp` | Jaeger, Grafana Tempo, any OTel collector |
| `jaeger` | Direct Jaeger HTTP export |

## Application Info

```go
import "github.com/enokdev/helix/observability"

info := observability.NewInfoProvider(loader,
    observability.WithVersion("1.2.3"),
    observability.WithBuildInfo(map[string]string{
        "commit": "abc123",
        "date":   "2024-01-15",
    }),
)
```

```json
// GET /actuator/info
{
  "version": "1.2.3",
  "profiles": ["prod"],
  "build": {
    "commit": "abc123",
    "date": "2024-01-15"
  }
}
```

## Complete Setup Example

```go
helix.Run(helix.App{
    Components: []any{
        &DatabaseHealthIndicator{db: db},
        &CacheHealthIndicator{client: redisClient},
    },
    Starters: []starter.Entry{
        {
            Name:    "observability",
            Order:   starter.OrderObservability,
            Starter: obsstarter.New(loader),
        },
    },
})
```

```yaml
# config/application.yaml
observability:
  enabled: true

logging:
  level: info

helix:
  starters:
    observability:
      enabled: true
```
