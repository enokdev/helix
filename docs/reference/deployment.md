# Deployment

Helix applications compile to a single static binary and can run anywhere Go binaries run: bare Linux servers, Docker containers, Kubernetes pods, systemd services.

Review the [Production Readiness](./production-readiness.md) checklist before exposing a Helix service to production traffic.

## Building a production binary

```bash
# Standard build (uses helix generate first, then go build)
helix build

# With build flags
helix build --output bin/my-api
```

The binary embeds no external dependencies — ship it and run it directly.

### Cross-compilation

```bash
# Linux amd64 (for Docker / most servers)
GOOS=linux GOARCH=amd64 helix build

# Linux arm64 (Apple Silicon servers, Raspberry Pi)
GOOS=linux GOARCH=arm64 helix build

# With version embedded in binary
helix build --ldflags="-X main.version=1.2.3 -X main.commit=$(git rev-parse --short HEAD)"
```

Access the injected values at runtime:

```go
var (
    version = "dev"   // overridden by -ldflags
    commit  = "none"
)

func main() {
    helix.Run(helix.App{
        Starters: []starter.Entry{{
            Starter: obsstarter.New(loader,
                obsstarter.WithVersion(version),
                obsstarter.WithBuildInfo(map[string]string{"commit": commit}),
            ),
        }},
    })
}
```

## Docker

### Generated Dockerfile

`helix build --docker` generates a multi-stage Dockerfile that produces a minimal final image:

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server .

# Runtime stage
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /app/server .
COPY config/ ./config/
EXPOSE 8080
ENTRYPOINT ["/app/server"]
```

Key points:
- `CGO_ENABLED=0` — fully static binary, no libc dependency
- `-ldflags="-s -w"` — strips debug info, reduces binary size (~30%)
- `distroless/static` — no shell, no package manager, minimal attack surface
- Config files are copied alongside the binary

### Docker Compose example

```yaml
# docker-compose.yml
services:
  api:
    build: .
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: "postgres://app:secret@db:5432/mydb"
      SECURITY_JWT_SECRET: "${JWT_SECRET}"
      HELIX_PROFILES_ACTIVE: "prod"
    depends_on:
      db:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/actuator/health"]
      interval: 15s
      timeout: 5s
      retries: 3
      start_period: 10s

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: mydb
      POSTGRES_USER: app
      POSTGRES_PASSWORD: secret
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U app -d mydb"]
      interval: 5s
      timeout: 3s
      retries: 5
    volumes:
      - pg_data:/var/lib/postgresql/data

volumes:
  pg_data:
```

## Environment variables in production

All configuration keys map to environment variables. Never put secrets in config files — use env vars.

### Mandatory production variables

| Variable | Config key | Purpose |
|----------|-----------|---------|
| `DATABASE_URL` | `database.url` | Database connection string |
| `SECURITY_JWT_SECRET` | `security.jwt.secret` | JWT signing key (min 32 chars) |
| `SERVER_PORT` | `server.port` | HTTP listen port (default `8080`) |
| `HELIX_PROFILES_ACTIVE` | — | Active config profiles (e.g. `prod`) |

### Optional production variables

| Variable | Config key | Recommended value |
|----------|-----------|-----------------|
| `HELIX_SHUTDOWN_TIMEOUT` | `helix.shutdown-timeout` | `30s` |
| `LOGGING_LEVEL` | `logging.level` | `warn` |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_ENABLED` | `helix.starters.observability.tracing.enabled` | `true` |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_EXPORTER` | `helix.starters.observability.tracing.exporter` | `otlp` |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_ENDPOINT` | `helix.starters.observability.tracing.endpoint` | Your OTel collector |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_SERVICE_NAME` | `helix.starters.observability.tracing.service-name` | `my-api` |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_INSECURE` | `helix.starters.observability.tracing.insecure` | `false` for TLS |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_HEADERS` | `helix.starters.observability.tracing.headers` | Comma-separated `key=value` OTLP headers |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_TLS_CA_FILE` | `helix.starters.observability.tracing.tls.ca-file` | PEM CA bundle |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_TLS_CERT_FILE` | `helix.starters.observability.tracing.tls.cert-file` | mTLS client certificate |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_TLS_KEY_FILE` | `helix.starters.observability.tracing.tls.key-file` | mTLS client key |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_TLS_SERVER_NAME` | `helix.starters.observability.tracing.tls.server-name` | TLS server name override |

### Generating a strong JWT secret

```bash
openssl rand -base64 48
```

## Production config file

Use a `application-prod.yaml` profile file for non-secret production settings:

```yaml
# config/application-prod.yaml
server:
  port: 8080

database:
  pool:
    max-open: 100
    max-idle: 10
    max-lifetime: 1h

logging:
  level: warn
  levels:
    "my-api/auth": info  # keep auth logs at info in prod

helix:
  shutdown-timeout: 30s
  starters:
    observability:
      tracing:
        enabled: true
        service-name: "my-api"
        exporter: otlp
        endpoint: "https://otel-collector:4318"
        insecure: false
        headers:
          x-tenant: "prod"
        tls:
          ca-file: "/etc/otel/ca.pem"
          cert-file: "/etc/otel/client.pem"
          key-file: "/etc/otel/client-key.pem"
          server-name: "otel-collector"
```

Activate it:

```bash
HELIX_PROFILES_ACTIVE=prod ./my-api
```

## Kubernetes

### Deployment manifest

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-api
  template:
    metadata:
      labels:
        app: my-api
    spec:
      containers:
        - name: my-api
          image: my-registry/my-api:1.2.3
          ports:
            - containerPort: 8080
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: my-api-secrets
                  key: database-url
            - name: SECURITY_JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: my-api-secrets
                  key: jwt-secret
            - name: HELIX_PROFILES_ACTIVE
              value: "prod"
          livenessProbe:
            httpGet:
              path: /actuator/health
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 15
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /actuator/health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
            failureThreshold: 2
          resources:
            requests:
              cpu: 100m
              memory: 64Mi
            limits:
              cpu: 500m
              memory: 256Mi
          lifecycle:
            preStop:
              exec:
                # Give k8s time to remove the pod from endpoints before shutdown
                command: ["sleep", "5"]
      terminationGracePeriodSeconds: 40  # must be > helix.shutdown-timeout
```

### Kubernetes Secrets

```bash
kubectl create secret generic my-api-secrets \
  --from-literal=database-url="postgres://app:pass@db:5432/mydb" \
  --from-literal=jwt-secret="$(openssl rand -base64 48)"
```

## Graceful shutdown best practices

Helix handles `SIGTERM` and `SIGINT` automatically:

1. Stop accepting new HTTP requests
2. Wait for in-flight requests to complete (up to `shutdown-timeout`)
3. Call `OnStop` on all lifecycle components in reverse startup order
4. Exit

### Recommended settings

```yaml
helix:
  shutdown-timeout: 30s   # give in-flight requests 30s to complete
```

In Kubernetes, set `terminationGracePeriodSeconds` to at least `shutdown-timeout + 10s` to account for the `preStop` sleep and pod removal delay.

### Verifying graceful shutdown

```bash
# Send SIGTERM and watch logs
kill -TERM $(pgrep my-api)
# Expected:
# {"level":"INFO","msg":"shutdown signal received"}
# {"level":"INFO","msg":"draining HTTP connections"}
# {"level":"INFO","msg":"database connection closed"}
# {"level":"INFO","msg":"shutdown complete","duration_ms":342}
```

## Health check integration

The `/actuator/health` endpoint returns:

- **200 OK** — all health indicators are `UP`
- **503 Service Unavailable** — at least one indicator is `DOWN`

This maps directly to Kubernetes readiness probes. When a component (e.g., database) is down, the pod is removed from the service load balancer until health is restored.

Add a custom health indicator for every external dependency your application talks to:

```go
type RedisHealthIndicator struct {
    client *redis.Client
}

func (h *RedisHealthIndicator) Name() string { return "redis" }

func (h *RedisHealthIndicator) Health(ctx context.Context) observability.ComponentHealth {
    if err := h.client.Ping(ctx).Err(); err != nil {
        return observability.ComponentHealth{Status: observability.StatusDown, Error: err.Error()}
    }
    return observability.ComponentHealth{Status: observability.StatusUp}
}
```

## Metrics scraping (Prometheus)

The `/actuator/metrics` endpoint exposes Prometheus metrics in text format. Configure your Prometheus scrape config:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: my-api
    static_configs:
      - targets: ["my-api:8080"]
    metrics_path: /actuator/metrics
    scrape_interval: 15s
```

For production, protect the metrics endpoint:

```go
observability.RegisterMetricsRoute(server, registry,
    observability.WithMetricsGuard(&InternalNetworkGuard{}),
)
```
