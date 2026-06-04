# Starters

Les starters sont des modules d'auto-configuration qui s'activent en fonction de vos dépendances `go.mod`, de votre fichier de config, ou de vos composants enregistrés. Ils éliminent le boilerplate en câblant l'infrastructure automatiquement.

## Comment fonctionnent les starters

Quand `helix.Run` démarre :

1. Tous les starters évaluent leur `Condition()` — la dépendance requise existe-t-elle ? La clé de config est-elle définie ?
2. Les starters qui passent la condition appellent `Configure(*core.Container)` pour enregistrer leurs composants
3. Les starters "marker-aware" scannent en plus le conteneur pour des types de composants spécifiques

Vous pouvez toujours surcharger avec une clé de config explicite.

## Starter Web

**Package :** `github.com/enokdev/helix/starter/web`

### Activation

| Déclencheur | Condition |
|-------------|-----------|
| `go.mod` | `github.com/gofiber/fiber/v2` présent |
| Clé de config | `helix.starters.web.enabled: true` |

Désactivation forcée : `helix.starters.web.enabled: false`

### Ce qu'il enregistre

| Composant | Type | Description |
|-----------|------|-------------|
| `web.HTTPServer` | `web.HTTPServer` | Serveur HTTP Fiber configuré sur `server.port` |
| `*serverLifecycle` | `core.Lifecycle` | Démarre/arrête le serveur dans le cycle de vie |

### Clés de config consommées

| Clé | Défaut | Description |
|-----|--------|-------------|
| `server.port` | `"8080"` | Port d'écoute |
| `helix.shutdown-timeout` | `"30s"` | Timeout de drainage à l'arrêt |

---

## Starter Data

**Package :** `github.com/enokdev/helix/starter/data`

### Activation

| Déclencheur | Condition |
|-------------|-----------|
| `go.mod` | `gorm.io/driver/sqlite` présent |
| Clé de config | `database.url` est définie |

Désactivation forcée : `helix.starters.data.enabled: false`

### Ce qu'il enregistre

| Composant | Type | Description |
|-----------|------|-------------|
| `*gorm.DB` | `*gorm.DB` | Instance de base de données GORM |
| `*datagorm.TransactionManager` | `*datagorm.TransactionManager` | Gestionnaire de transactions |
| `*datagorm.DB` | `*datagorm.DB` | Wrapper DB Helix avec cycle de vie |
| `*databaseLifecycle` | `core.Lifecycle` | Ping DB au démarrage, ferme à l'arrêt |

### Clés de config consommées

| Clé | Défaut | Description |
|-----|--------|-------------|
| `database.url` | — | Chemin de fichier SQLite ou chaîne de connexion |
| `database.pool.max-open` | `0` | Nombre max de connexions ouvertes |
| `database.pool.max-idle` | `0` | Nombre max de connexions inactives |
| `helix.starters.data.auto-migrate` | `false` | Exécuter GORM AutoMigrate au démarrage |

### Options

```go
import datastarter "github.com/enokdev/helix/starter/data"

datastarter.New(loader,
    datastarter.WithAutoMigrateModels(&User{}, &Order{}, &Product{}),
)
```

---

## Starter Security

**Package :** `github.com/enokdev/helix/starter/security`

### Activation

C'est un **MarkerAwareStarter** — il vérifie le conteneur *après* l'enregistrement des composants de l'app.

| Déclencheur | Condition |
|-------------|-----------|
| Section de config | Les clés `security.*` sont présentes |
| Clé de config | `helix.starters.security.enabled: true` |
| Marqueur de composant | `helix.SecurityConfigurer` trouvé dans le conteneur |

Désactivation forcée : `helix.starters.security.enabled: false`

### Ce qu'il enregistre

| Composant | Type | Description |
|-----------|------|-------------|
| `*security.JWTService` | `*security.JWTService` | Génération et validation JWT |

### Clés de config consommées

| Clé | Défaut | Description |
|-----|--------|-------------|
| `security.jwt.secret` | — | Clé de signature JWT (requise) |
| `security.jwt.expiry` | `"24h"` | Durée d'expiration du token |

---

## Starter Observability

**Package :** `github.com/enokdev/helix/starter/observability`

### Activation

| Déclencheur | Condition |
|-------------|-----------|
| Section de config | Les clés `observability.*` sont présentes |
| Clé de config | `helix.starters.observability.enabled: true` |

Désactivation forcée : `helix.starters.observability.enabled: false`

### Ce qu'il enregistre / configure

| Action | Description |
|--------|-------------|
| Logs structurés | Définit le `slog.Logger` global depuis la config `logging.*` |
| Tracing OTel | Configure un tracer provider quand `helix.starters.observability.tracing.enabled: true` |
| Health checker | Découvre tous les composants `HealthIndicator` et construit un `CompositeHealthChecker` |
| Routes actuator | Enregistre `/actuator/health`, `/actuator/info`, `/actuator/metrics` |
| Observer Prometheus | Installe `HTTPMetricsObserver` sur le serveur HTTP |
| `*observabilityLifecycle` | `core.Lifecycle` — arrête proprement le tracer provider OTel |

### Clés de config consommées

| Clé | Défaut | Description |
|-----|--------|-------------|
| `logging.level` | `"info"` | Niveau de log global |
| `logging.levels` | — | Surcharges par namespace |
| `helix.starters.observability.tracing.enabled` | `false` | Activer le tracing OTel |
| `helix.starters.observability.tracing.service-name` | `""` | Nom du service OTel |
| `helix.starters.observability.tracing.exporter` | `"stdout"` | `stdout` \| `otlp` \| `jaeger` |
| `helix.starters.observability.tracing.endpoint` | `""` | URL de l'endpoint exporteur |
| `helix.starters.observability.tracing.insecure` | `true` | Utiliser un transport OTLP non chiffré/non sécurisé pour compatibilité ascendante |
| `helix.starters.observability.tracing.headers` | — | Headers statiques de l'exporteur OTLP, par exemple auth ou tenant |
| `helix.starters.observability.tracing.tls.ca-file` | `""` | Bundle CA PEM pour le TLS OTLP |
| `helix.starters.observability.tracing.tls.cert-file` | `""` | Certificat client pour le mTLS |
| `helix.starters.observability.tracing.tls.key-file` | `""` | Clé du certificat client pour le mTLS |
| `helix.starters.observability.tracing.tls.server-name` | `""` | Surcharge du nom de serveur TLS |

---

## Starter Scheduling

**Package :** `github.com/enokdev/helix/starter/scheduling`

### Activation

C'est un **MarkerAwareStarter**.

| Déclencheur | Condition |
|-------------|-----------|
| `go.mod` | `github.com/robfig/cron/v3` présent |
| Marqueur de composant | `scheduler.ScheduledJobProvider` trouvé dans le conteneur |

### Ce qu'il enregistre

| Composant | Type | Description |
|-----------|------|-------------|
| `*scheduler.Scheduler` | `*scheduler.Scheduler` | Runner cron |
| `*scheduledJobRegistrar` | `core.Lifecycle` | Découvre les providers au démarrage, arrête le cron à l'arrêt |

---

## Priorité d'activation

Pour tout starter :

1. `helix.starters.<nom>.enabled: false` → **toujours inactif**, quelle que soit la condition
2. `helix.starters.<nom>.enabled: true` → **toujours actif**
3. Détection automatique (go.mod, sections de config, marqueurs de composants)

---

## Écrire un starter personnalisé

```go
type MyStarter struct {
    cfg config.Loader
}

func (s *MyStarter) Condition() bool {
    // Activer quand la clé de config est présente
    _, ok := s.cfg.Lookup("my-feature.enabled")
    return ok
}

func (s *MyStarter) Configure(container *core.Container) error {
    // Enregistrer vos composants
    return container.Register(&MyFeatureService{})
}

// Enregistrer avec helix.Run :
helix.Run(helix.App{
    Starters: []starter.Entry{
        {
            Name:    "my-feature",
            Order:   starter.OrderData + 1, // après le starter data
            Starter: &MyStarter{cfg: loader},
        },
    },
})
```

### Ordre d'exécution

| Constante | Valeur | Starter |
|-----------|--------|---------|
| `starter.OrderConfig` | 0 | Configuration (toujours en premier) |
| `starter.OrderWeb` | 1 | Web / Serveur HTTP |
| `starter.OrderData` | 2 | Base de données / repositories |
| `starter.OrderObservability` | 3 | Métriques, logging, tracing |
| `starter.OrderSecurity` | 4 | JWT, guards |
| `starter.OrderScheduling` | 5 | Planificateur cron |
