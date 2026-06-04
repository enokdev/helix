# Clés de configuration

Référence complète de toutes les clés de configuration reconnues par Helix et ses starters. Toutes les clés peuvent être surchargées avec des variables d'environnement en remplaçant `.` et `-` par `_` et en mettant en majuscules.

## Serveur

| Clé | Type | Défaut | Description |
|-----|------|--------|-------------|
| `server.port` | string | `"8080"` | Port d'écoute du serveur HTTP |

**Variable d'environnement :** `SERVER_PORT`

```yaml
server:
  port: 8080
```

## Helix Core

| Clé | Type | Défaut | Description |
|-----|------|--------|-------------|
| `helix.shutdown-timeout` | duration | `"30s"` | Temps maximum pour l'arrêt gracieux |

```yaml
helix:
  shutdown-timeout: 30s
```

## Rechargement de config

| Clé | Type | Défaut | Description |
|-----|------|--------|-------------|
| `helix.config.reload-interval` | duration | `""` | Intervalle de sondage de la config. Vide = désactivé |

```yaml
helix:
  config:
    reload-interval: 60s
```

## Starters

### Starter Web

| Clé | Type | Défaut | Description |
|-----|------|--------|-------------|
| `helix.starters.web.enabled` | bool | auto | Activer ou désactiver le starter web |

```yaml
helix:
  starters:
    web:
      enabled: true
```

### Starter Data

| Clé | Type | Défaut | Description |
|-----|------|--------|-------------|
| `helix.starters.data.enabled` | bool | auto | Activer ou désactiver le starter data |
| `helix.starters.data.auto-migrate` | bool | `false` | Exécuter l'auto-migration GORM au démarrage |

```yaml
helix:
  starters:
    data:
      enabled: true
      auto-migrate: true
```

### Starter Security

| Clé | Type | Défaut | Description |
|-----|------|--------|-------------|
| `helix.starters.security.enabled` | bool | auto | Activer ou désactiver le starter security |

```yaml
helix:
  starters:
    security:
      enabled: true
```

### Starter Observability

| Clé | Type | Défaut | Description |
|-----|------|--------|-------------|
| `helix.starters.observability.enabled` | bool | auto | Activer ou désactiver le starter observability |

```yaml
helix:
  starters:
    observability:
      enabled: true
```

## Base de données

| Clé | Type | Défaut | Description |
|-----|------|--------|-------------|
| `database.url` | string | — | URL de connexion à la base de données. Requis pour activer le starter data |
| `database.pool.max-open` | int | `0` (illimité) | Nombre maximum de connexions ouvertes |
| `database.pool.max-idle` | int | `0` | Nombre maximum de connexions inactives |

**Variables d'environnement :** `DATABASE_URL`, `DATABASE_POOL_MAX_OPEN`, `DATABASE_POOL_MAX_IDLE`

```yaml
database:
  url: "app.db"
  pool:
    max-open: 25
    max-idle: 5
```

**Exemples SQLite :**
```
"app.db"              # base de données fichier
":memory:"            # en mémoire (tests)
"file::memory:?cache=shared"  # en mémoire partagée
```

## Sécurité (JWT)

| Clé | Type | Défaut | Description |
|-----|------|--------|-------------|
| `security.jwt.secret` | string | — | Secret de signature JWT. Requis pour le service JWT |
| `security.jwt.expiry` | duration | `"24h"` | Durée d'expiration du token |

**Variables d'environnement :** `SECURITY_JWT_SECRET`, `SECURITY_JWT_EXPIRY`

```yaml
security:
  jwt:
    secret: "votre-clé-secrète"
    expiry: "24h"
```

::: warning Production
Définissez toujours `SECURITY_JWT_SECRET` via variable d'environnement en production. Ne commitez jamais de secrets dans le contrôle de version.
:::

## Logging

| Clé | Type | Défaut | Description |
|-----|------|--------|-------------|
| `logging.level` | string | `"info"` | Niveau de log global : `debug` \| `info` \| `warn` \| `error` |
| `logging.levels` | map[string]string | — | Surcharges de niveau de log par namespace |

```yaml
logging:
  level: info
  levels:
    "my-api/data": debug
    "my-api/web":  warn
    "my-api/auth": debug
```

## Observabilité / Tracing

| Clé | Type | Défaut | Description |
|-----|------|--------|-------------|
| `helix.starters.observability.tracing.enabled` | bool | `false` | Activer le tracing OpenTelemetry |
| `helix.starters.observability.tracing.service-name` | string | `""` | Nom du service OTel |
| `helix.starters.observability.tracing.exporter` | string | `"stdout"` | Exporteur : `stdout` \| `otlp` \| `jaeger` |
| `helix.starters.observability.tracing.endpoint` | string | `""` | URL de l'endpoint OTLP/Jaeger |
| `helix.starters.observability.tracing.insecure` | bool | `true` | Utiliser un transport OTLP non chiffré/non sécurisé pour compatibilité ascendante |
| `helix.starters.observability.tracing.headers` | map[string]string | — | Headers statiques de l'exporteur OTLP, par exemple auth ou tenant |
| `helix.starters.observability.tracing.tls.ca-file` | string | `""` | Bundle CA PEM pour le TLS OTLP |
| `helix.starters.observability.tracing.tls.cert-file` | string | `""` | Certificat client pour le mTLS |
| `helix.starters.observability.tracing.tls.key-file` | string | `""` | Clé du certificat client pour le mTLS |
| `helix.starters.observability.tracing.tls.server-name` | string | `""` | Surcharge du nom de serveur TLS |

```yaml
helix:
  starters:
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

## Infos de l'application

| Clé | Type | Défaut | Description |
|-----|------|--------|-------------|
| `app.name` | string | `""` | Nom de l'application (affiché dans `/actuator/info`) |
| `app.version` | string | `""` | Version de l'application |

```yaml
app:
  name: my-api
  version: "1.2.3"
```

## Exemple complet

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
    secret: "changez-moi"
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
