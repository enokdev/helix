# Déploiement

Les applications Helix compilent vers un binaire statique unique et peuvent s'exécuter partout où les binaires Go s'exécutent : serveurs Linux bare metal, conteneurs Docker, pods Kubernetes, services systemd.

Consultez la checklist [Readiness production](./production-readiness.md) avant d'exposer un service Helix au trafic de production.

## Construire un binaire de production

```bash
# Build standard (exécute helix generate d'abord, puis go build)
helix build

# Avec des flags de build
helix build --output bin/my-api
```

Le binaire n'embarque aucune dépendance externe — envoyez-le et exécutez-le directement.

### Compilation croisée

```bash
# Linux amd64 (pour Docker / la plupart des serveurs)
GOOS=linux GOARCH=amd64 helix build

# Linux arm64 (serveurs Apple Silicon, Raspberry Pi)
GOOS=linux GOARCH=arm64 helix build

# Avec version embarquée dans le binaire
helix build --ldflags="-X main.version=1.2.3 -X main.commit=$(git rev-parse --short HEAD)"
```

Accédez aux valeurs injectées à l'exécution :

```go
var (
    version = "dev"   // écrasé par -ldflags
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

### Dockerfile généré

`helix build --docker` génère un Dockerfile multi-étapes qui produit une image finale minimale :

```dockerfile
# Étape de build
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server .

# Étape d'exécution
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /app/server .
COPY config/ ./config/
EXPOSE 8080
ENTRYPOINT ["/app/server"]
```

Points clés :
- `CGO_ENABLED=0` — binaire entièrement statique, sans dépendance libc
- `-ldflags="-s -w"` — supprime les infos de debug, réduit la taille du binaire (~30%)
- `distroless/static` — sans shell, sans gestionnaire de paquets, surface d'attaque minimale
- Les fichiers de config sont copiés avec le binaire

### Exemple Docker Compose

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

## Variables d'environnement en production

Toutes les clés de configuration se mappent à des variables d'environnement. Ne mettez jamais de secrets dans les fichiers de config — utilisez des variables d'environnement.

### Variables de production obligatoires

| Variable | Clé de config | But |
|----------|--------------|-----|
| `DATABASE_URL` | `database.url` | Chaîne de connexion à la base de données |
| `SECURITY_JWT_SECRET` | `security.jwt.secret` | Clé de signature JWT (min 32 chars) |
| `SERVER_PORT` | `server.port` | Port d'écoute HTTP (défaut `8080`) |
| `HELIX_PROFILES_ACTIVE` | — | Profils de config actifs (ex. `prod`) |

### Variables de production optionnelles

| Variable | Clé de config | Valeur recommandée |
|----------|--------------|-------------------|
| `HELIX_SHUTDOWN_TIMEOUT` | `helix.shutdown-timeout` | `30s` |
| `LOGGING_LEVEL` | `logging.level` | `warn` |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_ENABLED` | `helix.starters.observability.tracing.enabled` | `true` |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_EXPORTER` | `helix.starters.observability.tracing.exporter` | `otlp` |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_ENDPOINT` | `helix.starters.observability.tracing.endpoint` | Votre collecteur OTel |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_SERVICE_NAME` | `helix.starters.observability.tracing.service-name` | `my-api` |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_INSECURE` | `helix.starters.observability.tracing.insecure` | `false` pour TLS |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_HEADERS` | `helix.starters.observability.tracing.headers` | Headers OTLP `key=value` séparés par des virgules |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_TLS_CA_FILE` | `helix.starters.observability.tracing.tls.ca-file` | Bundle CA PEM |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_TLS_CERT_FILE` | `helix.starters.observability.tracing.tls.cert-file` | Certificat client mTLS |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_TLS_KEY_FILE` | `helix.starters.observability.tracing.tls.key-file` | Clé client mTLS |
| `HELIX_STARTERS_OBSERVABILITY_TRACING_TLS_SERVER_NAME` | `helix.starters.observability.tracing.tls.server-name` | Surcharge du nom serveur TLS |

### Générer un secret JWT robuste

```bash
openssl rand -base64 48
```

## Fichier de config de production

Utilisez un fichier de profil `application-prod.yaml` pour les paramètres de production non-secrets :

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
    "my-api/auth": info  # garder les logs auth à info en prod

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

Activez-le :

```bash
HELIX_PROFILES_ACTIVE=prod ./my-api
```

## Kubernetes

### Manifest de Deployment

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
                # Donner à k8s le temps de retirer le pod des endpoints avant l'arrêt
                command: ["sleep", "5"]
      terminationGracePeriodSeconds: 40  # doit être > helix.shutdown-timeout
```

### Secrets Kubernetes

```bash
kubectl create secret generic my-api-secrets \
  --from-literal=database-url="postgres://app:pass@db:5432/mydb" \
  --from-literal=jwt-secret="$(openssl rand -base64 48)"
```

## Bonnes pratiques d'arrêt gracieux

Helix gère `SIGTERM` et `SIGINT` automatiquement :

1. Arrêter d'accepter de nouvelles requêtes HTTP
2. Attendre que les requêtes en vol se terminent (jusqu'à `shutdown-timeout`)
3. Appeler `OnStop` sur tous les composants du cycle de vie en ordre inverse
4. Quitter

### Paramètres recommandés

```yaml
helix:
  shutdown-timeout: 30s   # donner 30s aux requêtes en vol pour se terminer
```

Dans Kubernetes, définissez `terminationGracePeriodSeconds` à au moins `shutdown-timeout + 10s` pour tenir compte du sleep `preStop` et du délai de retrait du pod.

### Vérifier l'arrêt gracieux

```bash
# Envoyer SIGTERM et surveiller les logs
kill -TERM $(pgrep my-api)
# Attendu :
# {"level":"INFO","msg":"shutdown signal received"}
# {"level":"INFO","msg":"draining HTTP connections"}
# {"level":"INFO","msg":"database connection closed"}
# {"level":"INFO","msg":"shutdown complete","duration_ms":342}
```

## Intégration des health checks

L'endpoint `/actuator/health` retourne :

- **200 OK** — tous les indicateurs de santé sont `UP`
- **503 Service Unavailable** — au moins un indicateur est `DOWN`

Cela se mappe directement aux readiness probes Kubernetes. Quand un composant (ex. base de données) est down, le pod est retiré du load balancer du service jusqu'à ce que la santé soit restaurée.

Ajoutez un indicateur de santé personnalisé pour chaque dépendance externe avec laquelle votre application communique :

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

## Scraping des métriques (Prometheus)

L'endpoint `/actuator/metrics` expose les métriques Prometheus en format texte. Configurez votre scrape config Prometheus :

```yaml
# prometheus.yml
scrape_configs:
  - job_name: my-api
    static_configs:
      - targets: ["my-api:8080"]
    metrics_path: /actuator/metrics
    scrape_interval: 15s
```

En production, protégez l'endpoint des métriques :

```go
observability.RegisterMetricsRoute(server, registry,
    observability.WithMetricsGuard(&InternalNetworkGuard{}),
)
```
