---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
lastStep: 8
status: 'complete'
completedAt: '2026-04-14'
inputDocuments:
  - '_bmad-output/product-development/PRD.md'
  - '_bmad-output/product-development/product-brief.md'
workflowType: 'architecture'
project_name: 'helix'
user_name: 'Enokdev'
date: '2026-04-14'
---

# Architecture Decision Document — Helix

_Ce document se construit collaborativement, étape par étape. Les sections sont ajoutées au fil des décisions architecturales._

## Analyse du Contexte Projet

### Vue d'ensemble des exigences

**Exigences fonctionnelles :**
15 domaines fonctionnels couvrant : conteneur DI (reflection + codegen), couche web déclarative (routing convention + custom, extracteurs typés, mapping retour→HTTP), couche data (Repository générique [T,ID], query:"auto" codegen, AOP transactionnel), configuration YAML/ENV/profils + rechargement dynamique, cycle de vie applicatif (OnStart/OnStop, graceful shutdown), observabilité (/actuator/*), sécurité (JWT, RBAC), tests intégrés (NewTestApp, MockBean[T]), scheduling déclaratif, contextes domaine DDD-light, migrations CLI, CLI complet (new/generate/run/build).

**Exigences non fonctionnelles :**
- Go 1.21+ (requis pour slog)
- Démarrage < 100ms — contrainte de performance critique
- Latence P99 /actuator/health < 5ms
- Starters optionnels — zéro dépendance forcée
- Couverture tests core/ > 80%
- Onboarding < 30 min pour API CRUD complète

**Échelle & Complexité :**
- Domaine primaire : backend Go — framework/library
- Niveau de complexité : **Élevé** (deux modes DI, codegen, AOP, starters)
- Composants architecturaux estimés : 8–12 packages distincts

### Contraintes Techniques & Dépendances

- **Fiber** : dépendance HTTP principale, doit rester invisible derrière abstraction
- **GORM** (Phase 1), **Ent** (Phase 2), **sqlc** (Phase 3) : adaptateurs ORM
- **go/ast** : parsing compile-time pour query:"auto" et directives //helix:*
- **slog** (stdlib Go 1.21+) : logging structuré
- **Prometheus** : métriques, via client_golang
- **OpenTelemetry** : opt-in tracing

### Préoccupations Transversales Identifiées

1. **Génération de code** — impacte DI, HTTP routing, Data queries, AOP
2. **Résolution du graphe de dépendances** — ordre déterministe DI + lifecycle
3. **Convention de struct tags** — unifiée sur tout le framework (`inject:`, `value:`, `query:`, `mapstructure:`)
4. **Gestion d'erreurs explicite** — surface d'API cohérente, pas de panic
5. **Isolation des starters** — couplage conditionnel, pas de dépendances forcées

## Fondations du Projet & Outillage

### Domaine Technique Primaire

Backend Go — library/framework open-source. Pas de starter CLI externe applicable : Helix s'initialise via `go mod init`.

### Initialisation du Module

```bash
go mod init github.com/enokdev/helix   # ou github.com/{org}/helix
```

### Structure du Dépôt — Module Unique (Phase 1-2)

```
helix/
├── go.mod
├── go.sum
├── core/           # DI container, lifecycle, registry
├── web/            # Fiber integration, routing, middleware
├── data/           # Repository pattern, ORM adapters
├── config/         # YAML/ENV/profile loader
├── starter/        # Auto-configuration modules
├── observability/  # Prometheus, slog, OTel
├── cli/            # helix CLI tool (Phase 3)
├── testutil/       # Helpers NewTestApp, MockBean
└── internal/       # Packages privés au framework
```

> Migration vers `go.work` workspace prévue en Phase 3 quand `cli/` devient un binaire distribué indépendamment.

### Décisions Architecturales de Fondation

**Langage & Runtime :**
- Go 1.21+ (minimum absolu — requis pour `slog`, generics stables)

**Outillage de développement :**
- Linting : `golangci-lint` (vet + staticcheck + errcheck + gofumpt)
- Formatting : `gofmt` / `gofumpt`
- Code generation : `go generate` (entrée pour `//go:generate helix generate`)
- Tests : `go test ./...` (stdlib) + `testify/assert` + `testify/mock`
- CI : GitHub Actions (lint + test + build)

**Gestion des dépendances :**
- Fiber v2 (HTTP)
- GORM v2 (data Phase 1)
- Viper (config YAML/ENV)
- client_golang (Prometheus)
- go-opentelemetry (opt-in tracing)
- robfig/cron v3 (scheduling)

**Conventions de packages :**
- Pas de `package main` dans `core/` — tout est importable
- Interfaces publiques dans chaque package root (`core/container.go`)
- Implémentations dans sous-dossiers (`core/internal/`)
- `internal/` pour helpers non-exportés

**Note :** L'initialisation du module et la structure de base constituent la première story d'implémentation (Phase 1, Sprint 0).

## Décisions Architecturales Fondamentales

### Analyse des Priorités

**Décisions Critiques (bloquent l'implémentation) :**
- Architecture duale DI (Resolver abstrait)
- Moteur de génération de code (CLI + go:generate)
- Découverte des composants par mode
- Abstraction HTTP (helix.Context)

**Décisions Importantes (structurent l'architecture) :**
- Activation des starters (auto-detect + override YAML)

**Décisions Différées (post-MVP) :**
- Versioning des modules (go.work workspace) — Phase 3
- Support gRPC/WebSocket — hors périmètre Phase 1-2

---

### 1. Architecture Duale DI — Couche `Resolver` abstraite

**Décision :** Option C — Couche `Resolver` interchangeable

```go
// core/resolver.go
type Resolver interface {
    Resolve(target any) error
    Register(component any) error
    Graph() DependencyGraph
}

// Le Container délègue toute résolution au Resolver
type Container struct {
    resolver Resolver
    // ...
}

// Deux implémentations
type ReflectResolver struct { /* reflection runtime */ }
type WireResolver struct     { /* codegen compile-time */ }
```

**Rationale :** Permet d'ajouter un 3ème mode sans modifier le Container. Les deux modes partagent exactement la même API publique (`inject:"true"`, `value:"key"`). Le mode est sélectionné au démarrage via config ou flag `--reflect`.

**Impacts :** `core/`, `starter/`, `testutil/`

---

### 2. Moteur de Génération de Code — CLI + `go generate` wrapper

**Décision :** Option C — CLI standalone + wrapper `go:generate`

```bash
# CLI (source de vérité)
helix generate             # scan tout le projet
helix generate module user # génère un module complet

# Alias go:generate (appelle le CLI)
//go:generate helix generate
```

Le binaire `helix generate` :
1. Parse les packages via `go/ast` + `go/types`
2. Détecte les structs avec embeds `helix.Service`, `helix.Controller`, etc.
3. Détecte les directives `//helix:route`, `//helix:transactional`, `//helix:scheduled`
4. Détecte les interfaces taguées `query:"auto"`
5. Génère `helix_wire_gen.go` (wiring DI), route registration, query implementations, transaction wrappers

**Fichiers générés :** versionnables, lisibles, sans dépendance à la reflection en production.

**Impacts :** `cli/`, `core/internal/codegen/`, `data/`, `web/`

---

### 3. Découverte des Composants

**Décision :** Double stratégie selon le mode

**Mode Reflection (Phase 1 — défaut) :**
```go
helix.Run(App{
    Scan: []string{"./internal/...", "./cmd/..."},
})
// Auto-scan des packages déclarés → détection des embeds helix.Service etc.
```

**Mode Codegen (Phase 4 — opt-in) :**
```go
// helix generate produit helix_bootstrap.go :
func init() {
    container.Register(&UserService{})
    container.Register(&UserController{})
    // ... généré automatiquement
}
```

**Rationale :** Phase 1 priorise la DX Spring (auto-scan, zéro enregistrement manuel). Phase 4 bascule vers le zéro-magic compile-time sans changer les structs utilisateur.

**Impacts :** `core/`, `starter/web`, `starter/data`

---

### 4. Abstraction HTTP — `helix.Context` thin wrapper

**Décision :** Option C — Thin wrapper + helpers typés + codegen pour les extracteurs

```go
// web/context.go — jamais de *fiber.Ctx visible hors du package web/
type Context interface {
    Param(key string) string
    Header(key string) string
    IP() string
    // Pas d'accès direct à fiber.Ctx
}

// Les controllers ne voient que des types Helix
func (c *UserController) Index(params GetUsersParams) ([]User, error) {
    // params injecté automatiquement par helix — zero fiber.Ctx
}

// web/internal/fiber_adapter.go — seul endroit où fiber est importé
type fiberAdapter struct { app *fiber.App }
```

**Interface `HTTPServer` minimale :**
```go
type HTTPServer interface {
    Start(addr string) error
    Stop(ctx context.Context) error
    RegisterRoute(method, path string, handler HandlerFunc)
}
```

**Rationale :** Les controllers n'importent jamais `gofiber/fiber`. Un swap vers net/http ou fasthttp ne toucherait que `web/internal/`.

**Impacts :** `web/`, tous les controllers utilisateur

---

### 5. Activation des Starters — Auto-detect + Override YAML

**Décision :** Option C — Convention over configuration avec override explicite

**Logique d'activation :**
```yaml
# application.yaml — les starters s'activent automatiquement
# mais peuvent être surchargés :
helix:
  starters:
    web:
      enabled: true       # défaut: true si fiber dans go.mod
      port: 8080
    data:
      enabled: true       # défaut: true si driver DB dans go.mod + config database.*
    observability:
      enabled: false      # désactiver explicitement si besoin
```

**Détection automatique :**
- Starter `web` : `gofiber/fiber` présent dans `go.mod`
- Starter `data` : driver DB (`gorm.io/driver/*`) + clé `database.url` dans config
- Starter `observability` : clé `helix.starters.observability.enabled: true`
- Starter `security` : clé `security.*` présente dans config

**Rationale :** Comportement identique à Spring Boot (`@ConditionalOnClass`, `@ConditionalOnProperty`). Zéro configuration requise pour le cas standard.

**Impacts :** `starter/`, `config/`, `core/`

---

### Analyse d'Impact & Séquence d'Implémentation

**Séquence recommandée :**
1. `core/` — Container + Resolver interface + ReflectResolver
2. `config/` — Viper loader + profils + mapstructure
3. `web/` — HTTPServer interface + fiberAdapter + helix.Context
4. `starter/web` + `starter/config` — auto-configuration Phase 1
5. `data/` — Repository[T,ID] + GORMAdapter
6. `testutil/` — NewTestApp + MockBean[T]
7. `observability/` — /actuator/* endpoints
8. `cli/` — helix generate (codegen engine)

**Dépendances croisées :**
- `web/` dépend de `core/` (DI) et `config/` (port, middleware config)
- `data/` dépend de `core/` (injection) et `config/` (database.url)
- `starter/*` dépend de tous les packages qu'ils auto-configurent
- `testutil/` dépend de `core/` + remplace les starters par des mocks
- `cli/` est indépendant au runtime — outil de build uniquement

## Patterns d'Implémentation & Règles de Cohérence

### Points de Conflit Critiques Identifiés

7 zones où des agents IA pourraient faire des choix incompatibles : naming Go, organisation des packages, interfaces publiques, gestion d'erreurs, formats API, patterns de test, directives codegen.

---

### 1. Naming Patterns (Go)

**Interfaces — sans préfixe `I` ni suffixe `Interface` :**
```go
// CORRECT
type Container interface { ... }
type Resolver  interface { ... }
type Repository[T any, ID any] interface { ... }

// INTERDIT
type IContainer interface { ... }
type ContainerInterface interface { ... }
```

**Constructeurs — toujours `New` + nom du type retourné :**
```go
func NewContainer(opts ...Option) *Container { ... }
func NewReflectResolver() *ReflectResolver  { ... }
```

**Erreurs — sentinelles avec préfixe `Err`, types avec suffixe `Error` :**
```go
var ErrNotFound  = errors.New("helix: not found")
var ErrCyclicDep = errors.New("helix: cyclic dependency")

type ValidationError struct { Field string; Message string }
type NotFoundError   struct { Resource string; ID any }
```

**Packages — minuscules, singulier, pas de snake_case :**
```go
package core     // CORRECT
package web
package data
package testutil

package Core     // INTERDIT — majuscule
package configs  // INTERDIT — pluriel
package web_layer // INTERDIT — underscore
```

**Struct tags — valeurs canoniques :**
```go
Repo UserRepository `inject:"true"`       // CORRECT
Port int            `value:"server.port"` // CORRECT — point comme séparateur

Repo UserRepository `inject:"1"`          // INTERDIT
Port int            `value:"serverPort"`  // INTERDIT — utiliser kebab/dot notation
```

---

### 2. Organisation des Packages

**Structure interne de chaque package :**
```
core/
├── container.go       # interface Container + NewContainer
├── resolver.go        # interface Resolver
├── lifecycle.go       # interface Lifecycle
├── options.go         # types Option
├── errors.go          # toutes les erreurs du package
├── container_test.go  # tests co-localisés — JAMAIS dans sous-dossier test/
└── internal/
    ├── reflect_resolver.go
    └── wire_resolver.go
```

**Règles :**
- Tests TOUJOURS co-localisés (`*_test.go` dans le même package)
- `internal/` pour les implémentations non exportées
- Un seul fichier `errors.go` par package
- Interfaces publiques dans le fichier racine du package
- Fichiers générés : suffixe obligatoire `_gen.go` (ex: `helix_gen.go`)

---

### 3. Interfaces Publiques

**Toujours minimales — une responsabilité par interface :**
```go
type Repository[T any, ID any] interface {
    FindAll() ([]T, error)
    FindByID(id ID) (*T, error)
    Save(entity *T) error
    Delete(id ID) error
}

// Fonctionnalités optionnelles via interfaces composables séparées
type Paginatable[T any, ID any] interface {
    Paginate(page, size int) (Page[T], error)
}
type Transactable[T any, ID any] interface {
    WithTransaction(tx Transaction) Repository[T, ID]
}
```

---

### 4. Gestion d'Erreurs

**Wrapping — toujours `fmt.Errorf` avec `%w` et contexte `package: action:` :**
```go
return fmt.Errorf("core: resolve %T: %w", target, ErrCyclicDep)
return fmt.Errorf("web: register route GET /users: %w", err)
return fmt.Errorf("data: query FindByEmail: %w", err)
```

**Réponse d'erreur HTTP — format JSON structuré unique :**
```json
{
  "error": {
    "type":    "ValidationError",
    "message": "email is required",
    "field":   "email",
    "code":    "VALIDATION_FAILED"
  }
}
```

INTERDIT : `{"error": "message"}` (plat), `{"message": "..."}` (sans type)

---

### 5. Formats API & Réponses HTTP

**Réponse success — direct, pas de wrapper `data:` :**
```json
// GET /users/:id → 200
{ "id": 1, "name": "Alice", "email": "alice@example.com" }

// GET /users → 200 (liste paginée)
{ "items": [...], "total": 42, "page": 1, "pageSize": 20 }
```

**Conventions :**
- Champs JSON : `snake_case` (`created_at`, `user_id`)
- Dates : RFC3339 (`"2026-04-14T18:00:00Z"`)
- Routes : pluriel kebab-case (`/users`, `/blog-posts`)
- Paramètres de route : `:id` (jamais `:userId`)

---

### 6. Patterns de Test

**Table-driven tests obligatoires pour les cas multiples :**
```go
func TestContainer_Resolve(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(*Container)
        target  any
        wantErr error
    }{
        {name: "resolves singleton", ...},
        {name: "returns ErrNotFound for unregistered", wantErr: ErrNotFound},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) { ... })
    }
}
```

**Nommage : `Test{Type}_{Method}` :**
```
TestContainer_Register
TestContainer_Resolve
TestReflectResolver_DetectCycle
TestFiberAdapter_RegisterRoute
```

**Tests d'intégration — build tag `integration` + suffixe `_integration_test.go` :**
```go
//go:build integration
package core_test
```

---

### 7. Directives de Génération de Code

**Format canonique — deux slashes, pas d'espace, pas de `+` :**
```go
//helix:route GET /users/search   // CORRECT
//helix:guard authenticated
//helix:transactional
//helix:scheduled 0 0 * * *

// helix:route GET /users         // INTERDIT — espace
//+helix:route GET /users         // INTERDIT — préfixe +
```

**En-tête obligatoire des fichiers générés :**
```go
// Code generated by helix generate. DO NOT EDIT.
```

---

### Règles Obligatoires — Tous les Agents

**DOIVENT :**
1. Interfaces sans `I` — `New` constructeurs — `Err` préfixes pour les erreurs
2. Tests co-localisés en `*_test.go`, table-driven pour les cas multiples
3. `fmt.Errorf("package: action: %w", err)` pour tout wrapping d'erreur
4. Interfaces minimales — jamais de méthodes internes exportées
5. `snake_case` JSON, routes REST au pluriel, paramètre `:id`
6. `//helix:directive` sans espace ni préfixe
7. Ne JAMAIS modifier les fichiers `*_gen.go`

**Anti-patterns interdits :**
- `panic()` dans le framework (sauf init-time avec message explicite)
- Importer `gofiber/fiber` hors de `web/internal/`
- Logique métier dans les controllers
- `interface{}` au lieu de generics (Go 1.21+)
- `context.Context` stocké dans une struct (toujours en paramètre)

## Structure du Projet & Frontières Architecturales

### Arborescence Complète du Projet

```
helix/
├── go.mod                          # module github.com/{org}/helix, go 1.21
├── go.sum
├── README.md
├── CONTRIBUTING.md
├── LICENSE
├── .github/
│   └── workflows/
│       ├── ci.yml                  # lint + test + build sur push/PR
│       └── release.yml             # goreleaser sur tag v*
├── .golangci.yml
│
├── helix.go                        # helix.Run(), helix.App
│                                   # marqueurs : Service, Controller, Repository,
│                                   #   Component, ErrorHandler, SecurityConfigurer
│
├── core/                           # DI Container, Resolver, Lifecycle, Registry
│   ├── container.go                # interface Container + NewContainer
│   ├── resolver.go                 # interface Resolver
│   ├── lifecycle.go                # interface Lifecycle { OnStart() error; OnStop() error }
│   ├── registry.go                 # registre composants + scopes
│   ├── options.go                  # ContainerOption, WithMode, WithScan
│   ├── errors.go                   # ErrNotFound, ErrCyclicDep, ErrUnresolvable
│   ├── container_test.go
│   ├── resolver_test.go
│   └── internal/
│       ├── reflect_resolver.go
│       ├── reflect_resolver_test.go
│       ├── graph.go                # DependencyGraph — détection de cycles
│       └── graph_test.go
│
├── config/                         # YAML/ENV/Profils loader
│   ├── loader.go                   # interface Loader + NewLoader (Viper)
│   ├── profile.go                  # gestion HELIX_PROFILES_ACTIVE
│   ├── reload.go                   # interface ConfigReloadable + SIGHUP handler
│   ├── options.go
│   ├── errors.go                   # ErrConfigNotFound, ErrInvalidConfig
│   ├── loader_test.go
│   ├── profile_test.go
│   └── testdata/
│       ├── application.yaml
│       ├── application-test.yaml
│       └── application-dev.yaml
│
├── web/                            # Couche HTTP déclarative
│   ├── server.go                   # interface HTTPServer + NewServer
│   ├── context.go                  # interface Context
│   ├── router.go                   # interface Router
│   ├── handler.go                  # HandlerFunc, extracteurs typés
│   ├── response.go                 # OK(), Created(), NotFound(), BadRequest()...
│   ├── middleware.go               # interface Middleware
│   ├── guard.go                    # interface Guard — //helix:guard
│   ├── interceptor.go              # interface Interceptor — //helix:interceptor
│   ├── errors.go                   # HTTPError, mapping error → status code
│   ├── server_test.go
│   ├── handler_test.go
│   └── internal/
│       ├── fiber_adapter.go        # SEUL fichier qui importe gofiber/fiber
│       ├── fiber_context.go        # implémentation Context via *fiber.Ctx
│       ├── route_registry.go
│       └── fiber_adapter_test.go
│
├── data/                           # Repository Pattern + ORM adapters
│   ├── repository.go               # interface Repository[T, ID] + Page[T] + Filter
│   ├── transaction.go              # interface Transaction + TransactionManager
│   ├── pagination.go               # type Page[T]
│   ├── errors.go                   # ErrRecordNotFound, ErrDuplicateKey
│   ├── repository_test.go
│   └── gorm/
│       ├── adapter.go              # GORMRepository[T, ID]
│       ├── transaction.go
│       ├── adapter_test.go
│       └── testdata/
│
├── observability/                  # Prometheus, slog, OTel
│   ├── health.go                   # HealthIndicator + CompositeHealthChecker
│   ├── metrics.go                  # Prometheus handler + registry
│   ├── info.go                     # /actuator/info
│   ├── logging.go                  # slog setup par namespace
│   ├── tracing.go                  # OpenTelemetry (opt-in)
│   ├── errors.go
│   ├── health_test.go
│   └── metrics_test.go
│
├── starter/                        # Auto-configuration
│   ├── starter.go                  # interface Starter { Condition() bool; Configure(*core.Container) }
│   ├── web/starter.go
│   ├── data/starter.go
│   ├── config/starter.go
│   ├── observability/starter.go
│   └── security/starter.go         # Phase 3
│
├── security/                       # JWT, RBAC (Phase 3)
│   ├── configurer.go               # interface SecurityConfigurer + HttpSecurity builder
│   ├── jwt.go
│   ├── rbac.go
│   └── errors.go                   # ErrUnauthorized, ErrForbidden
│
├── testutil/                       # Helpers de test
│   ├── app.go                      # NewTestApp(t, ...opts) *TestApp
│   ├── mock.go                     # MockBean[T](impl T) Option
│   ├── bean.go                     # GetBean[T](app) T
│   ├── fixtures.go
│   └── app_test.go
│
├── cli/                            # Binaire helix (Phase 3)
│   ├── main.go
│   ├── cmd/
│   │   ├── root.go
│   │   ├── new.go                  # helix new app <nom>
│   │   ├── generate.go             # helix generate [module|context|repository]
│   │   ├── run.go                  # helix run (hot reload)
│   │   └── db/
│   │       └── migrate.go          # helix db migrate [create|up|down|status]
│   └── internal/
│       ├── codegen/
│       │   ├── scanner.go          # go/ast — détecte embeds + directives
│       │   ├── di_gen.go           # génère helix_gen.go
│       │   ├── route_gen.go
│       │   ├── query_gen.go        # génère SQL depuis query:"auto"
│       │   └── txn_gen.go          # génère wrappers //helix:transactional
│       └── scaffold/
│           └── templates/
│
└── examples/
    ├── crud-api/                   # API CRUD complète (Phase 1)
    │   ├── main.go
    │   ├── users/
    │   └── config/application.yaml
    └── domain-context/             # DDD-light (Phase 3)
```

---

### Frontières Architecturales

**Règle d'import stricte — flux à sens unique :**

```
cli/          → core, config, web, data  (build-time uniquement)
starter/*     → core, config + package configuré
web/          → core, config
data/         → core, config
observability → core, config, web
security/     → core, config, web
testutil/     → core, config, web, data, starter/*

web/internal/   ← seul package autorisé à importer gofiber/fiber
data/gorm/      ← seul package autorisé à importer gorm.io
```

**Interdictions absolues :**
- `core/` n'importe AUCUN autre package Helix (zéro dépendance cyclique)
- `config/` n'importe AUCUN autre package Helix
- Les `starter/*` ne s'importent pas entre eux

---

### Mapping Exigences PRD → Structure

| Exigence PRD | Package | Fichiers clés |
|---|---|---|
| `helix.Run()` + DI | `core/`, `helix.go` | `container.go`, `helix.go` |
| Embeds marqueurs | `helix.go` | types `Service`, `Controller`... |
| Config YAML/profils | `config/` | `loader.go`, `profile.go` |
| Routing + extracteurs typés | `web/` | `handler.go`, `router.go` |
| Isolation Fiber | `web/internal/` | `fiber_adapter.go` |
| Repository[T, ID] | `data/` | `repository.go` |
| GORM adapter | `data/gorm/` | `adapter.go` |
| `/actuator/*` | `observability/` | `health.go`, `metrics.go`, `info.go` |
| JWT + RBAC | `security/` | `jwt.go`, `rbac.go` |
| `NewTestApp` + `MockBean` | `testutil/` | `app.go`, `mock.go` |
| Starters auto-config | `starter/` | `starter.go` + sous-packages |
| `helix generate` codegen | `cli/internal/codegen/` | `scanner.go`, `*_gen.go` |
| Lifecycle + graceful shutdown | `core/` | `lifecycle.go` |

---

### Flux de Données Principal

```
helix.Run(App{})
  ├── ConfigStarter     → charge application.yaml + ENV
  ├── WebStarter        → crée fiberAdapter (fiber.App invisible)
  ├── DataStarter       → ouvre connexion DB + pool
  ├── ObsStarter        → enregistre /actuator/* routes
  └── Container.Resolve → wiring DI (reflect ou codegen)
       └── Lifecycle.OnStart → écoute :8080
            │
            └── [requête HTTP]
                 └── fiberAdapter → web.Context → HandlerFunc
                      └── Controller.Method(TypedParams)
                           └── Service.Method()
                                └── Repository.Query() → data/gorm
```

---

### Organisation des Tests

| Package | Type de test | Outil |
|---|---|---|
| `core/` | Unitaires — container + resolver | `go test`, `testify` |
| `config/` | Unitaires avec testdata/ YAML | `go test` |
| `web/` | Handlers + routing | `httptest`, `fiber.Test()` |
| `web/internal/` | Adapter Fiber | `fiber.Test()` |
| `data/gorm/` | Intégration (SQLite in-memory) | `go test -tags integration` |
| `testutil/` | Tests des helpers | `go test` |
| `starter/*/` | Conditions d'activation | `go test` |

## Résultats de Validation Architecturale

### Validation de Cohérence ✅

**Compatibilité des décisions :** Toutes les technologies choisies (Go 1.21+, Fiber v2, GORM v2, Viper, slog, Prometheus) sont compatibles sans conflit de version ni de paradigme.

**Consistance des patterns :** Naming Go uniforme, wrapping d'erreurs `fmt.Errorf("pkg: action: %w")` applicable partout, JSON `snake_case` cohérent avec les conventions GORM par défaut.

**Alignement structurel :** Frontières d'import vérifiables par `golangci-lint` (depguard). Isolation Fiber dans `web/internal/` respectée.

---

### Couverture des Exigences ✅ (22/22 couverts)

21 des 22 exigences fonctionnelles étaient couvertes. 1 écart critique (`//helix:scheduled`) résolu par ajout du package `scheduler/`.

---

### Écart Résolu : Package `scheduler/`

**Ajout à la structure :**

```
scheduler/
├── scheduler.go                # interface Scheduler + NewScheduler (robfig/cron v3)
├── job.go                      # type Job, CronExpression
├── errors.go                   # ErrJobNotFound, ErrInvalidCron
├── scheduler_test.go
└── internal/
    └── cron_adapter.go         # seul fichier à importer robfig/cron

starter/scheduling/
└── starter.go                  # actif si des jobs sont enregistrés au démarrage

cli/internal/codegen/
└── schedule_gen.go             # génère enregistrement des //helix:scheduled
```

**Dépendance :** `github.com/robfig/cron/v3`

---

### Checklist de Complétude

**Analyse du Contexte**
- [x] Contexte projet analysé et validé
- [x] Complexité évaluée : Élevée (framework Go)
- [x] Contraintes techniques identifiées
- [x] Préoccupations transversales mappées

**Décisions Architecturales**
- [x] Architecture DI duale — couche Resolver abstraite
- [x] Moteur codegen — CLI standalone + go:generate wrapper
- [x] Découverte composants — auto-scan (reflection) + bootstrap généré (codegen)
- [x] Abstraction HTTP — helix.Context thin wrapper, Fiber invisible
- [x] Activation starters — auto-detect + override YAML

**Patterns d'Implémentation**
- [x] Naming Go (interfaces, constructeurs, erreurs, packages, struct tags)
- [x] Organisation packages (co-location tests, internal/, errors.go unique)
- [x] Interfaces minimales + composition optionnelle
- [x] Gestion d'erreurs (fmt.Errorf + JSON structuré HTTP)
- [x] Formats API (snake_case, RFC3339, routes pluriel)
- [x] Patterns de test (table-driven, build tags integration)
- [x] Directives codegen (//helix:* format canonique)

**Structure du Projet**
- [x] Arborescence complète avec tous packages + fichiers clés
- [x] Frontières d'import strictes documentées
- [x] Mapping 22 exigences → structure complet
- [x] Flux de données principal documenté
- [x] Organisation des tests par package

---

### Évaluation de Maturité

**Statut global : PRÊT POUR L'IMPLÉMENTATION**

**Niveau de confiance : Élevé**

**Points forts :**
- Architecture duale DI élégante — Resolver abstrait, swap sans friction
- Isolation totale de Fiber dans `web/internal/` — swap futur sans impact sur les controllers
- Patterns de test clairs — table-driven, co-location, `MockBean[T]`
- Pipeline codegen bien défini — scanner AST → générateurs spécialisés
- Règles d'import vérifiables automatiquement via `golangci-lint`

**Axes d'amélioration futurs (post-Phase 2) :**
- Migration vers `go.work` workspace quand `cli/` devient binaire indépendant (Phase 3)
- Benchmarks CI automatisés pour valider démarrage < 100ms
- Système de plugins tiers (Phase 4)

---

### Première Priorité d'Implémentation

```bash
# Sprint 0 — Fondations
go mod init github.com/{org}/helix
# Créer : core/, config/, helix.go
# Objectif : helix.Run(App{}) qui démarre + container DI reflection basique
```
