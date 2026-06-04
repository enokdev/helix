# Observabilité

Helix inclut des logs structurés, des métriques Prometheus, du tracing OpenTelemetry et des endpoints actuator de style Spring — tout activé via le starter d'observabilité.

## Endpoints Actuator

Quand le starter d'observabilité est actif, trois endpoints sont enregistrés automatiquement :

| Endpoint | Description |
|----------|-------------|
| `GET /actuator/health` | Santé de l'application — 200 UP, 503 DOWN |
| `GET /actuator/metrics` | Métriques Prometheus en format texte |
| `GET /actuator/info` | Métadonnées de l'application (version, profils, build info) |

Activez le starter via la config :

```yaml
# config/application.yaml
observability:
  enabled: true
```

## Health checks

### Indicateurs de santé personnalisés

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

Enregistrez l'indicateur comme composant — le starter d'observabilité le découvre automatiquement :

```go
helix.Run(helix.App{
    Components: []any{
        &DatabaseHealthIndicator{db: db},
        // ...
    },
})
```

### Statuts des health checks

| Statut | Code HTTP | Signification |
|--------|-----------|--------------|
| `StatusUp` | 200 | Tous les checks passent |
| `StatusDown` | 503 | Au moins un check a échoué |
| `StatusUnknown` | 200 | Le check n'a pas pu déterminer l'état (ex. timeout) |

Retournez `StatusUnknown` quand une dépendance est injoignable mais pas définitivement en échec :

```go
func (h *ExternalAPIIndicator) Health(ctx context.Context) observability.ComponentHealth {
    checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()

    if err := h.ping(checkCtx); err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return observability.ComponentHealth{
                Status: observability.StatusUnknown,
                Error:  "health check en timeout",
            }
        }
        return observability.ComponentHealth{
            Status: observability.StatusDown,
            Error:  err.Error(),
        }
    }
    return observability.ComponentHealth{Status: observability.StatusUp}
}
```

### Format de la réponse health

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

Le statut global est `DOWN` (HTTP 503) si **un seul** composant est `DOWN`.

## Logs structurés

Helix utilise le package standard `log/slog` de Go avec sortie JSON par défaut.

### Configurer les logs

```go
import "github.com/enokdev/helix/observability"

logger, err := observability.ConfigureLogging(loader,
    observability.WithLoggingOutput(os.Stdout),
    observability.WithDefaultNamespace("my-api"),
)
```

### Configuration YAML

```yaml
logging:
  format: json     # json | text (défaut : json)
  level: info      # niveau global : debug | info | warn | error
  levels:
    "my-api/data": debug   # niveaux par namespace
    "my-api/web":  warn
```

### Format : JSON vs texte

```go
// JSON (production) :
// {"level":"INFO","time":"2024-01-15T10:00:00Z","msg":"utilisateur créé","id":42}

// Texte (développement) :
// 2024/01/15 10:00:00 INFO utilisateur créé id=42
```

### Loggers par namespace

```go
logger := observability.Logger("my-api/user")
logger.Info("utilisateur créé", "id", user.ID, "email", user.Email)
// {"level":"INFO","namespace":"my-api/user","msg":"utilisateur créé","id":42,"email":"alice@example.com"}
```

### Injecter le logger

```go
type UserService struct {
    helix.Service
    Log *slog.Logger `inject:"true"`
}
```

### Attributs structurés avec slog

```go
logger := observability.Logger("my-api/orders")

// Paires clé-valeur :
logger.Info("commande passée",
    "orderId", order.ID,
    "userId",  order.UserID,
    "total",   order.Total,
)

// Pré-attacher des champs avec With() :
reqLogger := logger.With("requestId", ctx.Header("X-Request-ID"), "userId", userID)
reqLogger.Info("traitement de la commande")
reqLogger.Info("commande expédiée")
// Les deux lignes de log incluent requestId et userId automatiquement
```

### WithGroup pour les attributs namespaced

```go
logger.Info("requête terminée",
    slog.Group("http",
        "method",  ctx.Method(),
        "path",    ctx.Path(),
        "status",  status,
        "latency", time.Since(start),
    ),
)
// {"level":"INFO","msg":"requête terminée","http":{"method":"POST","path":"/orders","status":201,"latency":"12ms"}}
```

## Métriques Prometheus

### Métriques auto-enregistrées

| Métrique | Type | Labels |
|---------|------|--------|
| `helix_http_requests_total` | Counter | `method`, `route`, `status` |
| `helix_http_request_duration_seconds` | Histogram | `method`, `route`, `status` |

### Métriques avancées

#### Histogram (distribution de latences)

```go
registry := observability.Registry()

dbQueryDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
    Name:    "db_query_duration_seconds",
    Help:    "Distribution de la latence des requêtes DB",
    Buckets: []float64{.001, .005, .01, .05, .1, .5, 1},
}, []string{"operation", "table"})

registry.MustRegister(dbQueryDuration)

// Utilisation :
start := time.Now()
_, err = repo.FindWhere(ctx, filter)
dbQueryDuration.With(prometheus.Labels{
    "operation": "find_where",
    "table":     "products",
}).Observe(time.Since(start).Seconds())
```

#### Gauge (état courant)

```go
activeConnections := prometheus.NewGauge(prometheus.GaugeOpts{
    Name: "ws_active_connections",
    Help: "Nombre de connexions WebSocket actives",
})

registry.MustRegister(activeConnections)

activeConnections.Inc()  // à la connexion
activeConnections.Dec()  // à la déconnexion
```

#### Counter (métriques manuelles)

```go
ordersTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "orders_total",
    Help: "Nombre total de commandes traitées",
}, []string{"status"})

registry.MustRegister(ordersTotal)

// Plus tard :
ordersTotal.With(prometheus.Labels{"status": "success"}).Inc()
```

## Tracing OpenTelemetry

### Configurer le tracing

```yaml
helix:
  starters:
    observability:
      tracing:
        enabled: true
        service-name: "my-api"
        exporter: otlp        # otlp | stdout | jaeger
        endpoint: "https://jaeger:4318"
        insecure: false
        headers:
          x-tenant: "dev"
        tls:
          ca-file: "/etc/otel/ca.pem"
          cert-file: "/etc/otel/client.pem"
          key-file: "/etc/otel/client-key.pem"
          server-name: "jaeger"
        sampling-ratio: 0.1   # tracer 10% des requêtes
```

### Exporteurs supportés

| Exporteur | Cas d'usage |
|-----------|------------|
| `stdout` | Développement / débogage |
| `otlp` | Jaeger, Grafana Tempo, tout collecteur OTel |
| `jaeger` | Export HTTP Jaeger direct |

## Informations de l'application

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

## Exemple de configuration complète

```yaml
# config/application.yaml
logging:
  format: json
  level: info
  levels:
    "my-api/data": debug  # logs DB verbeux en développement

helix:
  starters:
    observability:
      enabled: true
      tracing:
        enabled: false
```
