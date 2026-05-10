# Introduction

## Qu'est-ce que Helix ?

Helix est un **framework backend Go** construit sur [Fiber](https://gofiber.io/) qui apporte l'ergonomie de Spring Boot dans un Go idiomatique. Il vous fournit un squelette d'application prêt pour la production — injection de dépendances, routage par convention, auto-configuration, sécurité, observabilité et planification — afin que vous puissiez vous concentrer sur votre logique métier plutôt que sur le câblage de l'infrastructure.

<CustomCallout type="tip" title="Philosophie">
  <strong>Convention plutôt que configuration, explicite plutôt que magique.</strong> Helix rend les patterns communs triviaux tout en gardant tout débogable et remplaçable.
</CustomCallout>

## Pourquoi Helix ?

Construire une API REST Go from scratch signifie assembler un routeur, une stratégie DI, un chargeur de config, du middleware, des logs structurés, des health checks et des adaptateurs de base de données — avant d'écrire une seule ligne de logique métier. Des frameworks comme Gin ou Echo aident pour le routage, mais vous câblez encore tout vous-même.

Helix s'inspire du modèle d'**auto-configuration** de Spring Boot : déclarez vos composants, décrivez votre config, et le framework fait le reste.

## Principes de conception

| Principe | Ce que ça signifie |
|----------|-------------------|
| **Struct tags plutôt qu'annotations** | `inject:"true"` et `validate:"required"` — pas de magie de réflexion à la Java |
| **Compile-time plutôt que runtime** | Le mode Wire génère le code DI à la compilation ; la réflexion est un défaut opt-in |
| **Go idiomatique** | Pas d'état global, pas de surprises `init()`, câblage de dépendances explicite |
| **Performance** | Démarrage en moins de 100ms, faible latence API via le moteur fasthttp de Fiber |
| **Testabilité** | `NewTestApp` de première classe avec de vrais conteneurs, pas des mocks-de-mocks |

## Aperçu des fonctionnalités

### Injection de dépendances

Un conteneur DI type-safe avec injection automatique de champs via les tags `inject:"true"`. Supporte les scopes singleton et prototype, la détection de cycles, et deux modes de résolution :

- **ReflectResolver** — réflexion runtime, zéro configuration (défaut)
- **WireResolver** — génération de code à la compilation, zéro overhead runtime

### Routage par convention

Intégrez `helix.Controller` et nommez vos méthodes selon les conventions REST :

| Nom de méthode | Route HTTP |
|----------------|-----------|
| `Index()` | `GET /resource` |
| `Show()` | `GET /resource/:id` |
| `Create()` | `POST /resource` |
| `Update()` | `PUT /resource/:id` |
| `Delete()` | `DELETE /resource/:id` |

Surchargez avec des directives `//helix:route POST /auth/login`.

### Starters d'auto-configuration

Les starters s'activent automatiquement en fonction de vos dépendances `go.mod` et de votre fichier de config :

- **Web** — Serveur HTTP Fiber + gestion du cycle de vie
- **Data** — Connexion GORM + pattern repository
- **Security** — Service JWT + factories de guards
- **Observability** — Prometheus, slog, OpenTelemetry, endpoints actuator
- **Scheduling** — Runner cron avec arrêt gracieux

### Configuration

Chargeur basé sur Viper avec une chaîne de priorité claire :

```
Variables ENV  >  application-{profil}.yaml  >  application.yaml  >  défauts
```

Profils, rechargement dynamique sur SIGHUP, et liaison de structs `mapstructure` inclus.

## Comparaison

| Fonctionnalité | Raw Fiber | Echo | Helix |
|----------------|-----------|------|-------|
| Routage HTTP | ✓ | ✓ | ✓ Convention + custom |
| Injection de dépendances | ✗ | ✗ | ✓ Intégré |
| Auto-configuration | ✗ | ✗ | ✓ Starters |
| Gestion de config | ✗ | ✗ | ✓ YAML/ENV/profils |
| JWT + RBAC | ✗ | ✗ | ✓ Intégré |
| Endpoints health/métriques | ✗ | ✗ | ✓ Actuators |
| Planification cron | ✗ | ✗ | ✓ Intégré |
| Repository générique | ✗ | ✗ | ✓ Adaptateur GORM |
| Utilitaires TestApp | ✗ | ✗ | ✓ Intégré |

## Structure des packages

```
github.com/enokdev/helix
├── core/           # Conteneur DI, gestion du cycle de vie
├── web/            # Intégration HTTP Fiber, routage, middleware
├── data/           # Pattern repository, adaptateur GORM
├── config/         # Chargeur config YAML/ENV/TOML/JSON
├── starter/        # Modules d'auto-configuration
├── observability/  # Prometheus, slog, OpenTelemetry, actuators
├── security/       # JWT, RBAC, SecurityConfigurer
├── scheduler/      # Planification de tâches cron
└── cli/            # Générateur de projets/modules
```

## Modèle mental

Voici comment une requête traverse une application Helix :

```
helix.Run(App{Components: [...]})
         │
         ▼
  Conteneur DI construit
  (ReflectResolver résout les champs inject:"true")
         │
         ▼
  Lifecycle.OnStart() appelé
  (ping base de données, warmup cache, démarrage scheduler...)
         │
         ▼
  Serveur HTTP démarre sur :8080
         │
  ┌──────┴──────────────────────────────────────┐
  │  Requête entrante                           │
  │                                             │
  │  Guard.CanActivate()  ──échec──▶  401/403  │
  │         │ succès                            │
  │         ▼                                  │
  │  Interceptor.Intercept()  (avant)           │
  │         │                                  │
  │         ▼                                  │
  │  Handler(ctx, binding...) → (T, error)      │
  │         │                                  │
  │  Interceptor.Intercept()  (après)           │
  │         │                                  │
  │         ▼                                  │
  │  Erreur? → RequestError → réponse JSON     │
  │  T?      → sérialisation JSON → 200/201/204│
  └──────────────────────────────────────────────┘
         │
  SIGTERM/SIGINT
         │
         ▼
  Lifecycle.OnStop() appelé en ordre inverse
```

## Quand utiliser Helix (et quand ne pas l'utiliser)

**Utilisez Helix quand :**
- Vous construisez une API REST ou un microservice en Go et voulez des défauts sensés
- Vous voulez l'injection de dépendances sans un fichier de câblage séparé pour chaque service
- Votre équipe connaît Spring Boot et veut une ergonomie similaire en Go
- Vous avez besoin d'observabilité (métriques, health checks, tracing) sans ajouter 10 bibliothèques

**Envisagez des alternatives quand :**
- Votre app est un outil CLI, un batch job, ou une bibliothèque — Helix est centré sur le serveur HTTP
- Vous avez besoin d'un contrôle maximum sur chaque détail HTTP — Fiber brut vous donne plus de surface
- Votre équipe a déjà investi dans un framework DI spécifique (ex. `google/wire` seul)
- Vous écrivez une fonction serverless unique sans état long-running

## Migration depuis Fiber / Echo / Gin

Helix fait tourner Fiber en interne, donc le middleware et les handlers Fiber existants sont compatibles.

| Concept | Fiber / Echo / Gin | Helix |
|---------|--------------------|-------|
| Configuration du routeur | Appels `app.Get(...)` manuels | Méthodes convention + `//helix:route` |
| Middleware | `app.Use(...)` | `RegisterInterceptor` + `//helix:interceptor` |
| Injection de dépendances | Appels de constructeurs manuels | Tags `inject:"true"` |
| Config | `os.Getenv` / custom | `config.NewLoader` + YAML |
| Health check | Route custom | `/actuator/health` auto-enregistré |
| Guard d'auth | Middleware custom | `//helix:guard auth` |

## Prochaines étapes

- [Installation](/fr/guide/installation) — installer Helix et le CLI
- [Démarrage rapide](/fr/guide/quick-start) — construire votre première API en 5 minutes
- [Injection de dépendances](/fr/guide/dependency-injection) — comprendre le conteneur DI
- [DI avancé](/fr/guide/advanced-di) — mode Wire, injection d'interface, patterns collect
