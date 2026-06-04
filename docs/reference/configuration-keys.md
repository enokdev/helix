# Configuration Keys

Complete reference for all configuration keys recognized by Helix and its starters. All keys can be overridden with environment variables by replacing `.` and `-` with `_` and uppercasing.

## Server

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `server.port` | string | `"8080"` | HTTP server listen port |

**Environment variable:** `SERVER_PORT`

```yaml
server:
  port: 8080
```

## Helix Core

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `helix.shutdown-timeout` | duration | `"30s"` | Maximum time for graceful shutdown |

```yaml
helix:
  shutdown-timeout: 30s
```

## Config Reload

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `helix.config.reload-interval` | duration | `""` | Config polling interval. Empty = disabled |

```yaml
helix:
  config:
    reload-interval: 60s
```

## Starters

### Web Starter

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `helix.starters.web.enabled` | bool | auto | Enable or disable the web starter |

```yaml
helix:
  starters:
    web:
      enabled: true
```

### Data Starter

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `helix.starters.data.enabled` | bool | auto | Enable or disable the data starter |
| `helix.starters.data.auto-migrate` | bool | `false` | Run GORM auto-migration on startup |

```yaml
helix:
  starters:
    data:
      enabled: true
      auto-migrate: true
```

### Security Starter

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `helix.starters.security.enabled` | bool | auto | Enable or disable the security starter |

```yaml
helix:
  starters:
    security:
      enabled: true
```

### Observability Starter

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `helix.starters.observability.enabled` | bool | auto | Enable or disable the observability starter |

```yaml
helix:
  starters:
    observability:
      enabled: true
```

## Database

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `database.url` | string | — | Database connection URL. Required to activate the data starter |
| `database.pool.max-open` | int | `0` (unlimited) | Maximum open connections |
| `database.pool.max-idle` | int | `0` | Maximum idle connections |

**Environment variables:** `DATABASE_URL`, `DATABASE_POOL_MAX_OPEN`, `DATABASE_POOL_MAX_IDLE`

```yaml
database:
  url: "app.db"
  pool:
    max-open: 25
    max-idle: 5
```

**SQLite examples:**
```
"app.db"              # file database
":memory:"            # in-memory (tests)
"file::memory:?cache=shared"  # shared in-memory
```

## Security (JWT)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `security.jwt.secret` | string | — | JWT signing secret. Required for JWT service |
| `security.jwt.expiry` | duration | `"24h"` | Token expiry duration |

**Environment variables:** `SECURITY_JWT_SECRET`, `SECURITY_JWT_EXPIRY`

```yaml
security:
  jwt:
    secret: "your-secret-key"
    expiry: "24h"
```

::: warning Production
Always set `SECURITY_JWT_SECRET` via environment variable in production. Never commit secrets to version control.
:::

## Logging

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `logging.level` | string | `"info"` | Global log level: `debug` \| `info` \| `warn` \| `error` |
| `logging.levels` | map[string]string | — | Per-namespace log level overrides |

```yaml
logging:
  level: info
  levels:
    "my-api/data": debug
    "my-api/web":  warn
    "my-api/auth": debug
```

## Observability / Tracing

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `observability.tracing.enabled` | bool | `false` | Enable OpenTelemetry tracing |
| `observability.tracing.service-name` | string | `""` | OTel service name |
| `observability.tracing.exporter` | string | `"stdout"` | Exporter: `stdout` \| `otlp` \| `jaeger` |
| `observability.tracing.endpoint` | string | `""` | OTLP/Jaeger HTTP endpoint |
| `observability.tracing.insecure` | bool | `true` | Use plaintext/insecure OTLP transport for backward compatibility |
| `observability.tracing.headers` | map[string]string | — | Static OTLP exporter headers, for example auth or tenant headers |
| `observability.tracing.tls.ca-file` | string | `""` | PEM CA bundle for OTLP TLS |
| `observability.tracing.tls.cert-file` | string | `""` | Client certificate for mTLS |
| `observability.tracing.tls.key-file` | string | `""` | Client certificate key for mTLS |
| `observability.tracing.tls.server-name` | string | `""` | TLS server name override |

```yaml
observability:
  tracing:
    enabled: true
    service-name: "my-api"
    exporter: otlp
    endpoint: "https://otel-collector:4318"
    insecure: false
    headers:
      Authorization: "Bearer ${OTEL_TOKEN}"
    tls:
      ca-file: "/etc/ssl/otel-ca.pem"
      server-name: "otel-collector"
```

## App Info

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `app.name` | string | `""` | Application name (shown in `/actuator/info`) |
| `app.version` | string | `""` | Application version |

```yaml
app:
  name: my-api
  version: "1.2.3"
```

## Complete Example

```yaml
# config/application.yaml
server:
  port: 8080

app:
  name: my-api
  version: "1.0.0"

database:
  url: "my-api.db"
  pool:
    max-open: 25
    max-idle: 5

security:
  jwt:
    secret: "change-me"
    expiry: "24h"

logging:
  level: info
  levels:
    "my-api/data": debug

observability:
  tracing:
    enabled: false

helix:
  shutdown-timeout: 30s
  config:
    reload-interval: 60s
  starters:
    web:
      enabled: true
    data:
      enabled: true
      auto-migrate: true
    security:
      enabled: true
    observability:
      enabled: true
```

```yaml
# config/application-prod.yaml
database:
  url: "postgres://..."
  pool:
    max-open: 100
    max-idle: 10

logging:
  level: warn

observability:
  tracing:
    enabled: true
    service-name: "my-api"
    exporter: otlp
    endpoint: "http://otel-collector:4318"
```
