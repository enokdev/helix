---
stepsCompleted: [1, 2, 3, 4]
status: 'augmented'
completedAt: '2026-04-14'
augmentedAt: '2026-04-26'
inputDocuments:
  - '_bmad-output/product-development/PRD.md'
  - '_bmad-output/planning-artifacts/architecture.md'
augmentationSource: 'PM session Zero-Config vision (2026-04-26)'
---

# Helix - Epic Breakdown

## Overview

Ce document fournit la décomposition complète en epics et stories pour **Helix**, décomposant les exigences du PRD v2.1 et de l'Architecture en stories implémentables.

## Requirements Inventory

### Functional Requirements

FR1: Le système doit fournir un point d'entrée unique `helix.Run(App{})` qui auto-scanne les composants enregistrés, résout les dépendances et démarre le serveur HTTP.
FR2: Le système doit supporter l'injection de dépendances par reflection (mode défaut) avec détection automatique des composants via struct embeds (`helix.Service`, `helix.Controller`, `helix.Repository`, `helix.Component`).
FR3: Le système doit supporter l'injection de dépendances compile-time (mode opt-in) via génération de code (`helix generate`), produisant un fichier `helix_wire_gen.go` sans reflection en production.
FR4: Le conteneur DI doit détecter les cycles de dépendances au démarrage avec un message d'erreur explicite.
FR5: Le conteneur DI doit supporter les scopes Singleton (défaut) et Prototype, avec lazy loading configurable par composant.
FR6: Le système doit charger la configuration depuis des fichiers YAML (`application.yaml`, `application-{profile}.yaml`) avec priorité ENV > YAML profil actif > application.yaml > DEFAULT.
FR7: Le système doit supporter l'activation de profils via la variable d'environnement `HELIX_PROFILES_ACTIVE`.
FR8: Le système doit supporter le rechargement dynamique de la configuration via SIGHUP ou polling filesystem, avec callback `OnConfigReload()` pour les composants implémentant `helix.ConfigReloadable`.
FR9: Le système doit générer automatiquement des routes RESTful par convention de nommage des méthodes (Index→GET, Show→GET/:id, Create→POST, Update→PUT/:id, Delete→DELETE/:id).
FR10: Le système doit supporter des routes custom via directives de commentaire `//helix:route METHOD /path`.
FR11: Le système doit parser, valider et injecter automatiquement les paramètres de requête et le body typés dans les handlers.
FR12: Le système doit mapper automatiquement les types de retour des handlers vers les status HTTP appropriés (200/201/400/404/500).
FR13: Le système doit fournir un error handler centralisé via `helix.ErrorHandler` et directives `//helix:handles ErrorType`.
FR14: Le système doit implémenter une interface Repository générique `Repository[T any, ID any]` avec FindAll, FindByID, FindWhere, Save, Delete, Paginate, WithTransaction.
FR15: Le système doit générer automatiquement les implémentations SQL pour les méthodes d'interface taguées `query:"auto"` par analyse des noms de méthodes.
FR16: Le système doit supporter la gestion transactionnelle via directive `//helix:transactional` avec begin/commit/rollback automatique.
FR17: Le système doit implémenter les hooks de cycle de vie `OnStart() error` et `OnStop() error` avec graceful shutdown (timeout configurable, défaut 30s).
FR18: Le système doit exposer les endpoints `/actuator/health`, `/actuator/metrics` (Prometheus) et `/actuator/info`.
FR19: Le système doit supporter le logging structuré via `slog` avec niveau configurable par namespace et format JSON.
FR20: Le système doit fournir des métriques Prometheus natives et un support opt-in OpenTelemetry.
FR21: Le système doit fournir un module de sécurité avec JWT (génération, validation, refresh), RBAC guards déclaratifs, et `helix.SecurityConfigurer`.
FR22: Le système doit fournir des helpers de test `helix.NewTestApp()`, `helix.MockBean[T]()` et `helix.GetBean[T]()`.
FR23: Le système doit supporter les tâches planifiées via directive `//helix:scheduled cron` avec enregistrement automatique au démarrage.
FR24: Le système doit fournir une organisation par contextes de domaine DDD-light via `helix generate context <nom>`.
FR25: Le système doit fournir des commandes CLI pour les migrations DB (`helix db migrate create/up/down/status`).
FR26: Le système doit fournir un CLI complet (`helix new app`, `helix generate module/context/repository`, `helix run`, `helix build`).
FR27: Le système doit implémenter l'auto-configuration des starters (web, data, security, config, observability, scheduling) avec détection automatique et override YAML.
FR28: Le système doit supporter les guards déclaratifs (`//helix:guard`) et interceptors (`//helix:interceptor`) sur les routes.
FR-ZC1: `helix.Run()` doit fonctionner sans aucun argument — bootstrap complet automatique avec config auto-chargée depuis `config/application.yaml` et starters auto-détectés.
FR-ZC2: Les contrôleurs avec embed `helix.Controller` doivent être auto-découverts et auto-enregistrés sur le serveur HTTP par le starter web, sans appel manuel à `web.RegisterController()`.
FR-ZC3: Le lifecycle du serveur HTTP (démarrage/arrêt) doit être géré entièrement par le framework — le pattern `appServer{OnStart/OnStop}` ne doit plus être requis dans le code utilisateur.
FR-ZC4: Les starters doivent s'activer par présence de markers de composants dans le container (`helix.Controller`, `helix.SecurityConfigurer`, `//helix:scheduled`) en plus de la détection via `go.mod`.

### NonFunctional Requirements

NFR1: Version Go minimale Go 1.21 (requis pour slog et generics stables).
NFR2: Temps de démarrage < 100ms pour une application standard.
NFR3: Latence P99 `/actuator/health` < 5ms.
NFR4: Les dépendances externes doivent être optionnelles — chaque starter est une dépendance conditionnelle.
NFR5: Couverture de tests du package `core/` > 80%.
NFR6: Onboarding < 30 minutes pour une API CRUD complète (critère de validation Phase 1).

### Additional Requirements

- Initialisation : module Go unique `go mod init github.com/{org}/helix`, Go 1.21
- Structure de packages : `core/`, `web/`, `data/`, `config/`, `starter/`, `observability/`, `security/`, `scheduler/`, `testutil/`, `cli/`, `examples/`
- Isolation Fiber : `web/internal/` — seul package autorisé à importer `gofiber/fiber`
- Interface Resolver abstraite dans `core/` avec deux implémentations (`ReflectResolver`, `WireResolver`)
- Règles d'import strictes : `core/` et `config/` sans dépendances vers d'autres packages Helix
- Outillage CI : `golangci-lint`, `gofmt/gofumpt`, GitHub Actions (lint + test + build)
- Tests co-localisés en `*_test.go` dans chaque package, table-driven tests obligatoires
- Fichiers générés suffixés `_gen.go` — ne jamais modifier manuellement
- Directives codegen format canonique : `//helix:directive` sans espace

### UX Design Requirements

_Non applicable — Helix est un framework backend Go sans interface utilisateur._

### FR Coverage Map

FR1: Epic 1 — helix.Run() point d'entrée unique
FR2: Epic 1 — DI reflection + auto-scan embeds
FR4: Epic 1 — Détection cycles de dépendances
FR5: Epic 1 — Scopes Singleton/Prototype + lazy loading
FR17: Epic 1 — Lifecycle OnStart/OnStop + graceful shutdown
FR6: Epic 2 — YAML loader + priorité ENV > profil > défaut
FR7: Epic 2 — Profils via HELIX_PROFILES_ACTIVE
FR8: Epic 2 — Rechargement dynamique config (SIGHUP/polling)
FR9: Epic 3 — Routing par convention de nommage (Index/Show/Create/Update/Delete)
FR10: Epic 3 — Routes custom via //helix:route
FR11: Epic 3 — Extracteurs typés + validation automatique
FR12: Epic 3 — Mapping type retour → HTTP status
FR13: Epic 3 — Error handler centralisé //helix:handles
FR28: Epic 3 — Guards & Interceptors déclaratifs
FR14: Epic 4 — Repository[T, ID] générique (GORM)
FR15: Epic 4 — query:"auto" codegen SQL depuis noms de méthodes
FR16: Epic 4 — //helix:transactional AOP begin/commit/rollback
FR22: Epic 5 — NewTestApp + MockBean[T] + GetBean[T]
FR18: Epic 6 — /actuator/health, /actuator/metrics, /actuator/info
FR19: Epic 6 — slog logging structuré JSON par namespace
FR20: Epic 6 — Prometheus natif + OpenTelemetry opt-in
FR27: Epic 7 — Starters auto-configuration (web, data, config, observability)
FR21: Epic 8 — JWT + RBAC + SecurityConfigurer
FR23: Epic 9 — //helix:scheduled cron jobs auto-enregistrés
FR3: Epic 10 — DI codegen compile-time (helix generate wire)
FR24: Epic 10 — DDD contexts via helix generate context
FR25: Epic 10 — DB migrations CLI (helix db migrate)
FR26: Epic 10 — CLI complet (new/generate/run/build)
FR-ZC1: Epic 1 — helix.Run() zéro-paramètre, bootstrap complet automatique
FR-ZC2: Epic 7 — Auto-registration des contrôleurs par le starter web
FR-ZC3: Epic 7 — Lifecycle HTTP intégré, suppression du wrapper appServer
FR-ZC4: Epic 7 — Auto-détection des starters par markers de composants

## Epic List

### Epic 1: Application Bootstrap & DI Container
Un développeur peut créer une application Helix avec injection de dépendances automatique en une ligne de code — jusqu'à `func main() { helix.Run() }` en zéro-config complet.
**FRs couverts :** FR1, FR2, FR4, FR5, FR17, FR-ZC1
**Phase PRD :** Phase 1 (MVP)

### Epic 2: Configuration Centralisée
Un développeur peut centraliser sa configuration via YAML avec support de profils et rechargement dynamique.
**FRs couverts :** FR6, FR7, FR8
**Phase PRD :** Phase 1 (MVP)

### Epic 3: Couche HTTP Déclarative
Un développeur peut exposer une API REST complète via des conventions de nommage et des directives — zéro boilerplate de routing.
**FRs couverts :** FR9, FR10, FR11, FR12, FR13, FR28
**Phase PRD :** Phase 1 (FR9, FR10, FR11, FR12) + Phase 2 (FR13, FR28)

### Epic 4: Data Layer & Repository Pattern
Un développeur peut accéder à une base de données avec des repositories génériques typés, sans écrire de SQL.
**FRs couverts :** FR14, FR15, FR16
**Phase PRD :** Phase 1 (FR14) + Phase 2 (FR15, FR16)

### Epic 5: Infrastructure de Test
Un développeur peut tester son application Helix avec un container complet et des mocks automatiques.
**FRs couverts :** FR22
**Phase PRD :** Phase 1 (MVP)

### Epic 6: Observabilité
Un développeur peut monitorer son application Helix en production via des endpoints standardisés, des métriques Prometheus et du logging structuré.
**FRs couverts :** FR18, FR19, FR20
**Phase PRD :** Phase 2

### Epic 7: Auto-Configuration & Starters
Un développeur peut démarrer une application Helix sans aucune configuration explicite — les starters s'activent automatiquement, les contrôleurs sont auto-enregistrés et le lifecycle serveur est géré par le framework.
**FRs couverts :** FR27, FR-ZC2, FR-ZC3, FR-ZC4
**Phase PRD :** Phase 2

### Epic 8: Sécurité
Un développeur peut sécuriser son API avec JWT, RBAC et une configuration de sécurité déclarative.
**FRs couverts :** FR21
**Phase PRD :** Phase 3

### Epic 9: Scheduling
Un développeur peut planifier des tâches récurrentes de façon déclarative via une directive cron.
**FRs couverts :** FR23
**Phase PRD :** Phase 3

### Epic 10: CLI & Génération de Code
Un développeur peut scaffolder un projet, générer des modules, gérer les migrations et construire du code compile-time depuis le terminal.
**FRs couverts :** FR3, FR24, FR25, FR26
**Phase PRD :** Phase 3

### Epic 11: Documentation & Guides Développeur
Un développeur peut apprendre et maîtriser Helix en moins de 30 minutes grâce à une documentation complète et des guides pratiques.
**FRs couverts :** NFR6
**Phase PRD :** Phase 4 (Post-MVP)

### Epic 12: Exemples d'Applications
Un développeur peut partir d'un exemple concret et fonctionnel pour bootstrap son projet Helix.
**FRs couverts :** NFR6
**Phase PRD :** Phase 4 (Post-MVP)

### Epic 13: Assainissement Technique (Dette)
Le framework est exempt de data races, de bugs de sécurité connus et de limitations majeures documentées dans deferred-work.md.
**FRs couverts :** NFR1, NFR2, NFR4
**Phase PRD :** Phase 4 (Post-MVP)

---

## Epic 1: Application Bootstrap & DI Container

Un développeur peut créer une application Helix avec injection de dépendances automatique en une ligne de code.

### Story 1.1: Initialisation du Projet & Structure de Base

En tant que **contributeur du framework**,
Je veux initialiser le module Go Helix avec la structure de packages et l'outillage CI,
Afin d'avoir une base solide sur laquelle construire le framework.

**Acceptance Criteria:**

**Given** un répertoire vide
**When** `go mod init github.com/{org}/helix` est exécuté et les fichiers initiaux sont créés
**Then** le module compile avec `go build ./...` sans erreur
**And** `golangci-lint run` passe sans erreur
**And** `go test ./...` s'exécute (0 tests, 0 erreurs)
**And** GitHub Actions CI tourne sur push (lint + test + build)
**And** la structure de dossiers `core/`, `config/`, `web/`, `data/`, `testutil/` existe

### Story 1.2: Interfaces Publiques du Conteneur DI

En tant que **développeur utilisant Helix**,
Je veux une interface `Container` claire pour enregistrer et résoudre des dépendances,
Afin de comprendre le contrat du DI sans lire l'implémentation.

**Acceptance Criteria:**

**Given** le package `core/` importé
**When** un développeur consulte les types exportés
**Then** l'interface `Container` expose `Register(component any) error` et `Resolve(target any) error`
**And** l'interface `Resolver` est définie avec `Register`, `Resolve`, `Graph() DependencyGraph`
**And** `NewContainer(opts ...Option) *Container` crée un container fonctionnel
**And** les erreurs `ErrNotFound`, `ErrCyclicDep`, `ErrUnresolvable` sont exportées dans `errors.go`
**And** `core/` n'importe aucun autre package Helix

### Story 1.3: ReflectResolver — Enregistrement & Résolution Singleton

En tant que **développeur utilisant Helix**,
Je veux enregistrer mes structs et les résoudre automatiquement par type,
Afin de ne pas câbler mes dépendances manuellement.

**Acceptance Criteria:**

**Given** un `Container` avec `ReflectResolver`
**When** `container.Register(&UserService{})` est appelé
**Then** la struct est stockée dans le registre avec scope Singleton
**When** `container.Resolve(&service)` est appelé
**Then** la même instance est retournée (Singleton)
**And** les champs tagués `inject:"true"` sont injectés automatiquement
**And** les champs tagués `value:"server.port"` sont résolus depuis la config
**And** une erreur `ErrNotFound` est retournée pour un type non enregistré

### Story 1.4: Détection des Cycles de Dépendances

En tant que **développeur utilisant Helix**,
Je veux être alerté immédiatement si mes dépendances forment un cycle,
Afin de corriger le problème avant de déployer.

**Acceptance Criteria:**

**Given** deux services `A` dépendant de `B` et `B` dépendant de `A`
**When** `helix.Run()` est appelé
**Then** l'application refuse de démarrer
**And** l'erreur retournée est `ErrCyclicDep` avec le chemin du cycle (`A → B → A`)
**And** le message est lisible sans consulter la documentation
**And** aucune goroutine ou ressource n'est laissée ouverte

### Story 1.5: Scope Prototype & Lazy Loading

En tant que **développeur utilisant Helix**,
Je veux contrôler le scope d'instanciation de mes composants,
Afin d'obtenir une nouvelle instance à chaque résolution quand nécessaire.

**Acceptance Criteria:**

**Given** un composant enregistré avec `scope:"prototype"`
**When** `container.Resolve()` est appelé deux fois
**Then** deux instances différentes sont retournées
**Given** un composant configuré avec `lazy:"true"`
**When** le container démarre
**Then** le composant n'est pas instancié jusqu'à sa première résolution
**And** le Singleton reste le comportement par défaut (sans tag explicite)

### Story 1.6: Hooks de Cycle de Vie & Graceful Shutdown

En tant que **développeur utilisant Helix**,
Je veux que mon application démarre et s'arrête proprement,
Afin de ne pas perdre de requêtes en cours lors d'un redémarrage.

**Acceptance Criteria:**

**Given** un composant implémentant `Lifecycle` (OnStart/OnStop)
**When** `helix.Run()` est appelé
**Then** `OnStart()` est appelé dans l'ordre du graphe de dépendances
**When** SIGTERM ou SIGINT est reçu
**Then** `OnStop()` est appelé dans l'ordre inverse de l'initialisation
**And** le shutdown attend la fin des requêtes en cours (max 30s par défaut)
**And** le timeout est configurable via `helix.shutdown-timeout` dans application.yaml
**And** si `OnStop()` retourne une erreur, elle est loggée mais le shutdown continue

### Story 1.7: Point d'entrée `helix.Run(App{})` & Marqueurs de Composants

En tant que **développeur utilisant Helix**,
Je veux démarrer mon application en une seule ligne sans configurer le container manuellement,
Afin de me concentrer sur ma logique métier.

**Acceptance Criteria:**

**Given** une struct `App` avec `Scan: []string{"./internal/..."}` passée à `helix.Run()`
**When** l'application démarre
**Then** tous les packages listés sont scannés pour les composants Helix
**And** les structs avec embed `helix.Service`, `helix.Controller`, `helix.Repository`, `helix.Component` sont auto-enregistrés
**And** les dépendances sont résolues avant le premier `OnStart()`
**And** `helix.Run()` bloque jusqu'à réception de SIGTERM/SIGINT
**And** l'application démarre en < 100ms (benchmark CI)

### Story 1.8: helix.Run() Zéro-Paramètre — Bootstrap Complet Automatique

En tant que **développeur utilisant Helix**,
Je veux démarrer mon application avec `helix.Run()` sans aucun argument,
Afin que mon `main.go` se résume à `func main() { helix.Run() }`.

**Acceptance Criteria:**

**Given** un projet Helix avec des composants annotés et un fichier `config/application.yaml` ou `application.yaml` présent
**When** `helix.Run()` est appelé sans aucun argument
**Then** la configuration est chargée automatiquement depuis `config/application.yaml` (ou `application.yaml` en fallback)
**And** les starters sont auto-détectés : web si `gofiber/fiber` dans `go.mod`, security si clé `security.*` présente dans la config, data si driver DB dans `go.mod` et clé `database.url` présente
**And** tous les composants avec embed `helix.Controller`, `helix.Service`, `helix.Repository`, `helix.Component`, `helix.SecurityConfigurer` sont découverts et câblés automatiquement
**And** le serveur HTTP démarre sans aucune ligne de code de bootstrap manuel dans `main.go`
**And** si aucun fichier de config n'est trouvé, les valeurs par défaut des starters s'appliquent (port 8080, etc.)
**Given** `helix.Run(App{Starters: ..., Components: ...})` est utilisé par un développeur existant
**Then** le comportement existant est conservé sans aucune modification requise (rétrocompatibilité totale)
**And** l'application démarre en < 100ms (benchmark CI)

---

## Epic 2: Configuration Centralisée

Un développeur peut centraliser sa configuration via YAML avec support de profils et rechargement dynamique.

### Story 2.1: Loader YAML & Chaîne de Priorité

En tant que **développeur utilisant Helix**,
Je veux que ma configuration soit chargée automatiquement depuis `application.yaml`,
Afin de centraliser tous mes paramètres sans code de chargement manuel.

**Acceptance Criteria:**

**Given** un fichier `config/application.yaml` présent dans le répertoire de travail
**When** l'application démarre
**Then** la configuration est chargée automatiquement via Viper
**And** la priorité de résolution est respectée : ENV > profil actif YAML > application.yaml > valeur DEFAULT
**And** `container.Resolve(&cfg)` injecte la config dans les structs taguées `mapstructure:"..."`
**And** une variable d'environnement `SERVER_PORT=9090` surcharge `server.port: 8080` dans le YAML
**And** une erreur `ErrConfigNotFound` est retournée si aucun fichier de config n'est trouvé

### Story 2.2: Profils de Configuration

En tant que **développeur utilisant Helix**,
Je veux activer différents fichiers de configuration selon l'environnement,
Afin d'avoir des configurations distinctes pour dev, staging et production.

**Acceptance Criteria:**

**Given** les fichiers `application.yaml` et `application-prod.yaml` existent
**When** `HELIX_PROFILES_ACTIVE=prod` est défini
**Then** les valeurs de `application-prod.yaml` surchargent celles de `application.yaml`
**And** les clés absentes de `application-prod.yaml` sont héritées de `application.yaml`
**When** `HELIX_PROFILES_ACTIVE` n'est pas défini
**Then** seul `application.yaml` est chargé
**And** plusieurs profils peuvent être activés en les séparant par des virgules (`dev,local`)

### Story 2.3: Rechargement Dynamique de la Configuration

En tant que **développeur utilisant Helix**,
Je veux que ma configuration soit rechargée sans redémarrer l'application,
Afin de modifier des paramètres en production sans interruption de service.

**Acceptance Criteria:**

**Given** un composant implémentant `helix.ConfigReloadable`
**When** un signal `SIGHUP` est reçu par le processus
**Then** les fichiers YAML sont relus et la config en mémoire est mise à jour
**And** `OnConfigReload()` est appelé sur tous les composants `ConfigReloadable` enregistrés
**When** le polling filesystem est configuré (`helix.config.reload-interval: 30s`)
**Then** les changements de fichiers sont détectés automatiquement à l'intervalle configuré
**And** le rechargement est opt-in (désactivé par défaut)
**And** les erreurs de parsing lors du rechargement sont loggées sans crasher l'application

---

## Epic 3: Couche HTTP Déclarative

Un développeur peut exposer une API REST complète via des conventions de nommage et des directives — zéro boilerplate de routing.

### Story 3.1: Abstraction HTTP & Adaptateur Fiber

En tant que **développeur utilisant Helix**,
Je veux utiliser un serveur HTTP sans importer Fiber directement,
Afin que mon code de controller soit indépendant du framework HTTP sous-jacent.

**Acceptance Criteria:**

**Given** le package `web/` importé
**When** un controller est défini avec l'embed `helix.Controller`
**Then** le controller ne dépend jamais de `gofiber/fiber` dans ses imports
**And** `web.NewServer(opts ...Option) HTTPServer` crée un serveur Fiber encapsulé
**And** `web/internal/fiber_adapter.go` est le seul fichier qui importe `gofiber/fiber`
**And** `web.Context` expose `Param()`, `Header()`, `IP()` sans exposer `*fiber.Ctx`
**And** les tests de controller utilisent `httptest` sans dépendance à Fiber

### Story 3.2: Routing par Convention de Nommage

En tant que **développeur utilisant Helix**,
Je veux que mes méthodes de controller soient automatiquement mappées à des routes REST,
Afin de ne pas déclarer chaque route manuellement.

**Acceptance Criteria:**

**Given** un controller avec l'embed `helix.Controller` et des méthodes nommées par convention
**When** l'application démarre
**Then** `Index()` est enregistré sur `GET /users`
**And** `Show()` est enregistré sur `GET /users/:id`
**And** `Create()` est enregistré sur `POST /users`
**And** `Update()` est enregistré sur `PUT /users/:id`
**And** `Delete()` est enregistré sur `DELETE /users/:id`
**And** le préfixe de route est dérivé du nom du controller (`UserController` → `/users`)

### Story 3.3: Routes Custom via Directives

En tant que **développeur utilisant Helix**,
Je veux déclarer des routes non-conventionnelles via une directive de commentaire,
Afin de couvrir les cas où la convention ne suffit pas.

**Acceptance Criteria:**

**Given** une méthode annotée `//helix:route GET /users/search`
**When** l'application démarre
**Then** la méthode est enregistrée sur `GET /users/search`
**And** plusieurs directives sur la même méthode sont toutes enregistrées
**And** une directive avec espace (`// helix:route`) génère une erreur de parsing lisible
**And** une méthode sans directive convention ni `//helix:route` n'est pas enregistrée comme route

### Story 3.4: Extracteurs Typés & Validation Automatique

En tant que **développeur utilisant Helix**,
Je veux recevoir mes paramètres de requête déjà parsés et validés dans mes handlers,
Afin d'éliminer le boilerplate de parsing et de validation.

**Acceptance Criteria:**

**Given** un handler avec un paramètre struct taguée `query:` ou un body taguée `json:`
**When** une requête HTTP arrive
**Then** Helix parse et injecte automatiquement les query params dans la struct
**And** Helix parse et injecte automatiquement le body JSON dans la struct
**And** les tags `validate:"required,email"` sont vérifiés avant d'appeler le handler
**And** une validation échouée retourne automatiquement un `400` avec le détail de l'erreur
**And** les valeurs `default:"1"` et `max:"100"` sont appliquées sur les query params

### Story 3.5: Mapping Automatique Retour → HTTP Status

En tant que **développeur utilisant Helix**,
Je veux que le status HTTP soit déterminé automatiquement depuis le type de retour de mon handler,
Afin de me concentrer sur la logique et non sur les codes HTTP.

**Acceptance Criteria:**

**Given** un handler GET retournant `(*User, nil)`
**When** la requête est traitée
**Then** la réponse est `200 OK` avec le body JSON de `User`
**Given** un handler POST retournant `(*User, nil)`
**Then** la réponse est `201 Created` avec le body JSON
**Given** un handler retournant `(nil, helix.NotFoundError{})`
**Then** la réponse est `404 Not Found` avec JSON structuré `{"error": {"type": "NotFoundError", ...}}`
**Given** un handler retournant `(nil, helix.ValidationError{})`
**Then** la réponse est `400 Bad Request`
**Given** un handler retournant `(nil, error)` générique
**Then** la réponse est `500 Internal Server Error`

### Story 3.6: Error Handler Centralisé

En tant que **développeur utilisant Helix**,
Je veux définir des handlers d'erreurs globaux par type d'erreur,
Afin que mes controllers ne gèrent que le happy path.

**Acceptance Criteria:**

**Given** une struct avec embed `helix.ErrorHandler` et méthode annotée `//helix:handles ValidationError`
**When** n'importe quel handler de l'application retourne une `ValidationError`
**Then** le handler centralisé est invoqué automatiquement
**And** la réponse HTTP est celle retournée par le handler centralisé
**And** plusieurs types d'erreur peuvent être gérés dans le même `ErrorHandler`
**And** un type d'erreur sans handler enregistré est géré par le handler par défaut (500)

### Story 3.7: Guards & Interceptors Déclaratifs

En tant que **développeur utilisant Helix**,
Je veux protéger mes routes avec des guards et des interceptors via des directives,
Afin de sécuriser et enrichir mes endpoints sans répéter de la logique dans chaque handler.

**Acceptance Criteria:**

**Given** une route annotée `//helix:guard authenticated`
**When** une requête non authentifiée arrive
**Then** le guard intercepte la requête et retourne `401 Unauthorized` avant d'appeler le handler
**Given** une route annotée `//helix:guard role:admin`
**When** un utilisateur authentifié sans rôle admin arrive
**Then** la réponse est `403 Forbidden`
**Given** une route annotée `//helix:interceptor cache:5m`
**When** la même requête est effectuée deux fois
**Then** la deuxième réponse est servie depuis le cache sans appeler le handler
**And** les guards et interceptors sont chainables (plusieurs directives sur la même route)

---

## Epic 4: Data Layer & Repository Pattern

Un développeur peut accéder à une base de données avec des repositories génériques typés, sans écrire de SQL.

### Story 4.1: Interface Repository Générique

En tant que **développeur utilisant Helix**,
Je veux une interface repository typée que je peux implémenter ou faire implémenter par Helix,
Afin d'interagir avec ma base de données de façon cohérente et type-safe.

**Acceptance Criteria:**

**Given** le package `data/` importé
**When** un développeur déclare `type UserRepository interface { helix.Repository[User, int] }`
**Then** l'interface expose `FindAll()`, `FindByID(id int)`, `FindWhere(filter Filter)`, `Save(*User)`, `Delete(int)`, `Paginate(page, size int)`, `WithTransaction(tx Transaction)`
**And** `Page[T]` retourne `Items []T`, `Total int`, `Page int`, `PageSize int`
**And** `Filter` permet de construire des conditions de requête sans SQL brut
**And** `data/` n'importe pas directement `gorm.io` — seul `data/gorm/` le fait

### Story 4.2: Adaptateur GORM — Implémentation Repository

En tant que **développeur utilisant Helix**,
Je veux une implémentation GORM prête à l'emploi de `Repository[T, ID]`,
Afin de persister mes entités sans écrire de code de persistance.

**Acceptance Criteria:**

**Given** une entité `User` avec les tags GORM standards
**When** `data/gorm.NewRepository[User, int](db)` est utilisé
**Then** `FindAll()` retourne tous les enregistrements
**And** `FindByID(1)` retourne l'enregistrement avec `id = 1` ou `ErrRecordNotFound`
**And** `Save(&user)` crée ou met à jour l'enregistrement (upsert)
**And** `Delete(1)` supprime l'enregistrement avec `id = 1`
**And** `Paginate(2, 20)` retourne la page 2 avec 20 éléments et le total correct
**And** les tests d'intégration utilisent SQLite in-memory avec le build tag `integration`

### Story 4.3: Génération Automatique de Requêtes (`query:"auto"`)

En tant que **développeur utilisant Helix**,
Je veux déclarer des méthodes de recherche par leur nom et les avoir implémentées automatiquement,
Afin d'obtenir l'équivalent Go de Spring Data sans écrire de SQL.

**Acceptance Criteria:**

**Given** une interface repository avec `FindByEmail(email string) (*User, error)` taguée `query:"auto"`
**When** `helix generate` est exécuté
**Then** un fichier `*_gen.go` est généré avec l'implémentation SQL correspondante
**And** `FindByNameContaining(name string)` génère `WHERE name LIKE '%name%'`
**And** `FindByAgeGreaterThan(age int)` génère `WHERE age > age`
**And** `FindByEmailAndAge(email string, age int)` génère `WHERE email = ? AND age = ?`
**And** `FindAllOrderByCreatedAtDesc()` génère `ORDER BY created_at DESC`
**And** les fichiers générés commencent par `// Code generated by helix generate. DO NOT EDIT.`

### Story 4.4: Transactions Déclaratives (`//helix:transactional`)

En tant que **développeur utilisant Helix**,
Je veux que mes méthodes de service soient exécutées dans une transaction sans code de gestion explicite,
Afin de garantir la cohérence des données sans boilerplate try/catch/rollback.

**Acceptance Criteria:**

**Given** une méthode de service annotée `//helix:transactional`
**When** `helix generate` est exécuté
**Then** un wrapper généré démarre une transaction avant l'appel de la méthode
**And** la transaction est committée si la méthode retourne `nil`
**And** la transaction est rollbackée si la méthode retourne une `error != nil`
**And** le wrapper injecte la transaction dans les repositories via `WithTransaction(tx)`
**And** les appels imbriqués à des méthodes `//helix:transactional` participent à la transaction existante (propagation REQUIRED)

---

## Epic 5: Infrastructure de Test

Un développeur peut tester son application Helix avec un container complet et des mocks automatiques.

### Story 5.1: `helix.NewTestApp()` — Container de Test

En tant que **développeur utilisant Helix**,
Je veux démarrer un container Helix complet dans mes tests,
Afin de tester mes services avec leurs vraies dépendances sans infrastructure externe.

**Acceptance Criteria:**

**Given** un test Go standard avec `helix.NewTestApp(t)`
**When** le test s'exécute
**Then** un container DI complet est démarré dans le contexte du test
**And** le container est automatiquement arrêté via `t.Cleanup()` à la fin du test
**And** la configuration de test est chargée depuis `config/application-test.yaml` si présent
**And** `helix.GetBean[UserService](app)` retourne le composant résolu depuis le container
**And** `NewTestApp` accepte des options pour surcharger des composants

### Story 5.2: `helix.MockBean[T]()` — Remplacement Automatique par Mock

En tant que **développeur utilisant Helix**,
Je veux remplacer n'importe quel composant par un mock dans mes tests,
Afin d'isoler l'unité testée de ses dépendances externes.

**Acceptance Criteria:**

**Given** un test avec `helix.NewTestApp(t, helix.MockBean[UserRepository](mockRepo))`
**When** `helix.GetBean[UserService](app)` est appelé
**Then** le `UserService` reçoit `mockRepo` au lieu de l'implémentation réelle
**And** plusieurs `MockBean` peuvent être passés au même `NewTestApp`
**And** `helix.MockBean[T]` accepte n'importe quelle implémentation de l'interface `T`
**And** les autres composants non mockés sont résolus normalement depuis le container
**And** une erreur claire est retournée si le type `T` n'est pas enregistré dans le container

---

## Epic 6: Observabilité

Un développeur peut monitorer son application Helix en production via des endpoints standardisés, des métriques Prometheus et du logging structuré.

### Story 6.1: Endpoints Actuator (`/actuator/health`, `/actuator/info`)

En tant que **développeur utilisant Helix**,
Je veux des endpoints de santé et d'information standardisés,
Afin que mon infrastructure puisse vérifier l'état de l'application automatiquement.

**Acceptance Criteria:**

**Given** le starter `observability` actif
**When** `GET /actuator/health` est appelé
**Then** la réponse est `200 OK` avec `{"status": "UP"}` si tous les indicateurs sont sains
**And** la réponse est `503 Service Unavailable` avec `{"status": "DOWN", "components": {...}}` si un indicateur échoue
**And** les composants implémentant `HealthIndicator` sont automatiquement inclus dans le check
**When** `GET /actuator/info` est appelé
**Then** la réponse retourne la version, le profil actif et les build infos
**And** la latence P99 de `/actuator/health` est < 5ms (benchmark CI)

### Story 6.2: Métriques Prometheus (`/actuator/metrics`)

En tant que **développeur utilisant Helix**,
Je veux exposer des métriques Prometheus sans configuration manuelle,
Afin que mon système de monitoring les scrape automatiquement.

**Acceptance Criteria:**

**Given** le starter `observability` actif
**When** `GET /actuator/metrics` est appelé
**Then** les métriques Prometheus sont retournées au format text/plain standard
**And** les métriques HTTP par défaut sont exposées : `helix_http_requests_total`, `helix_http_request_duration_seconds`
**And** un développeur peut enregistrer des métriques custom via `observability.Registry()`
**And** le endpoint est protégeable via guard si nécessaire

### Story 6.3: Logging Structuré avec `slog`

En tant que **développeur utilisant Helix**,
Je veux un logging structuré JSON configuré automatiquement,
Afin de pouvoir interroger mes logs en production avec des outils comme Loki ou CloudWatch.

**Acceptance Criteria:**

**Given** l'application démarre
**Then** `slog` est configuré automatiquement avec le format JSON
**And** le niveau de log global est configurable via `helix.logging.level: debug|info|warn|error`
**And** le niveau par namespace est configurable via `helix.logging.levels.web: debug`
**And** chaque log structuré inclut `timestamp`, `level`, `msg`, et le namespace source
**And** `slog.Default()` retourne le logger Helix configuré, utilisable partout dans l'application

### Story 6.4: Tracing OpenTelemetry (opt-in)

En tant que **développeur utilisant Helix**,
Je veux activer le tracing distribué sans modifier mon code applicatif,
Afin de diagnostiquer les problèmes de performance dans un système distribué.

**Acceptance Criteria:**

**Given** `helix.starters.observability.tracing.enabled: true` dans la config
**When** une requête HTTP arrive
**Then** un span OpenTelemetry est créé automatiquement avec le nom de la route
**And** le trace context est propagé via les headers `traceparent` / `tracestate`
**And** l'exporter est configurable via `helix.starters.observability.tracing.exporter: otlp|jaeger|stdout`
**When** `helix.starters.observability.tracing.enabled: false` (défaut)
**Then** aucune dépendance OTel n'est chargée (zéro overhead)

---

## Epic 7: Auto-Configuration & Starters

Un développeur peut démarrer une application Helix sans aucune configuration explicite — les starters s'activent automatiquement.

### Story 7.1: Interface Starter & Mécanisme de Condition

En tant que **développeur utilisant Helix**,
Je veux que les modules du framework s'activent automatiquement selon le contexte,
Afin de ne configurer que ce qui s'écarte des conventions.

**Acceptance Criteria:**

**Given** l'interface `Starter` avec `Condition() bool` et `Configure(*core.Container)`
**When** `helix.Run()` orchestre le démarrage
**Then** chaque starter est évalué — `Condition()` détermine s'il s'active
**And** les starters actifs sont configurés dans l'ordre : config → web → data → observability → security → scheduling
**And** un starter désactivé ne charge aucune dépendance
**And** le résultat d'activation de chaque starter est loggé au niveau `debug`

### Story 7.2: Starter Web — Auto-activation Fiber

En tant que **développeur utilisant Helix**,
Je veux que le serveur HTTP démarre automatiquement si Fiber est dans mes dépendances,
Afin de ne pas déclarer explicitement le starter web.

**Acceptance Criteria:**

**Given** `gofiber/fiber` est présent dans `go.mod`
**When** `helix.Run()` est appelé sans config `helix.starters.web.enabled`
**Then** le starter web s'active automatiquement
**And** le serveur écoute sur le port défini dans `server.port` (défaut : `8080`)
**Given** `helix.starters.web.enabled: false` dans la config
**Then** le starter web ne s'active pas, même si Fiber est dans `go.mod`

### Story 7.3: Starter Data — Auto-activation DB

En tant que **développeur utilisant Helix**,
Je veux que la connexion à la base de données soit établie automatiquement,
Afin de ne pas écrire de code de connexion dans mon application.

**Acceptance Criteria:**

**Given** un driver GORM dans `go.mod` et `database.url` défini dans la config
**When** `helix.Run()` est appelé
**Then** le starter data s'active et établit la connexion DB via GORM
**And** le pool de connexions est configuré via `database.pool.max-open`, `database.pool.max-idle`
**And** la connexion est vérifiée au démarrage — l'application refuse de démarrer si la DB est inaccessible
**And** `helix.starters.data.auto-migrate: true` exécute les auto-migrations GORM au démarrage (opt-in)

### Story 7.4: Starters Observability, Security & Scheduling

En tant que **développeur utilisant Helix**,
Je veux activer les modules transversaux via la configuration,
Afin d'enrichir mon application sans modifier le code de bootstrap.

**Acceptance Criteria:**

**Given** `helix.starters.observability.enabled: true` (ou clé `observability.*` présente)
**When** l'application démarre
**Then** les endpoints `/actuator/*` sont enregistrés et le logging slog est configuré
**Given** `helix.starters.security.enabled: true` (ou clé `security.*` présente)
**Then** le middleware JWT est installé et `SecurityConfigurer` est appliqué
**Given** des composants avec `//helix:scheduled` sont enregistrés
**Then** le starter scheduling s'active automatiquement et enregistre les jobs cron
**And** chaque starter peut être forcé à `enabled: false` pour désactiver l'auto-détection

### Story 7.5: Auto-Registration des Contrôleurs par le Starter Web

En tant que **développeur utilisant Helix**,
Je veux que le starter web découvre et enregistre automatiquement tous mes contrôleurs,
Afin de ne plus écrire `web.RegisterController(server, ctrl)` pour chaque contrôleur.

**Acceptance Criteria:**

**Given** des structs avec embed `helix.Controller` enregistrées dans le container DI
**When** le starter web s'active
**Then** chaque contrôleur est découvert automatiquement depuis le container
**And** les routes de chaque contrôleur sont enregistrées sur le serveur HTTP sans aucun code utilisateur
**And** les guard factories requises par les directives `//helix:guard` des contrôleurs sont auto-enregistrées (`role` guard factory activée si `//helix:guard role:*` est détecté)
**And** l'ordre de registration est déterministe et suit l'ordre d'enregistrement dans le container
**Given** un projet avec `AuthController`, `APIController` et `AdminController` dans le container
**Then** les trois contrôleurs et toutes leurs routes sont actifs sans une seule ligne de code de registration manuel
**And** si un contrôleur est résolu avec une erreur, `helix.Run()` échoue avec un message identifiant le contrôleur concerné

### Story 7.6: Lifecycle HTTP Intégré — Suppression du Wrapper appServer

En tant que **développeur utilisant Helix**,
Je veux que le serveur HTTP démarre et s'arrête sans que j'aie à créer un wrapper lifecycle,
Afin de supprimer définitivement le pattern `appServer{OnStart/OnStop}` de mon code.

**Acceptance Criteria:**

**Given** le starter web actif et `server.port` configuré dans `application.yaml` (défaut: 8080)
**When** `helix.Run()` est appelé
**Then** le serveur HTTP démarre automatiquement via le lifecycle interne du starter web
**And** aucune struct `appServer` implémentant `OnStart/OnStop` n'est nécessaire dans le code utilisateur
**And** l'adresse d'écoute est construite depuis `server.port` sans code utilisateur
**When** SIGTERM ou SIGINT est reçu
**Then** le serveur s'arrête proprement via le graceful shutdown du container
**And** les requêtes en cours sont finalisées avant l'arrêt (timeout `helix.shutdown-timeout`, défaut 30s)
**And** le pattern `appServer` reste disponible comme option avancée pour les cas nécessitant un contrôle fin du lifecycle (rétrocompatibilité)

### Story 7.7: Auto-Détection des Starters par Markers de Composants

En tant que **développeur utilisant Helix**,
Je veux que les starters s'activent automatiquement selon les composants présents dans mon code,
Afin de ne jamais déclarer explicitement `Starters: []starter.Entry{starter.Security()}` dans `main.go`.

**Acceptance Criteria:**

**Given** un composant avec embed `helix.SecurityConfigurer` est dans le container DI
**When** `helix.Run()` orchestre le démarrage
**Then** le starter security s'active automatiquement — sans déclaration explicite dans `main.go` ni clé `security.*` dans la config
**And** le service JWT est créé depuis `security.jwt.secret` et `security.jwt.expiry` dans la config
**Given** des composants avec directive `//helix:scheduled` sont détectés dans le container
**Then** le starter scheduling s'active automatiquement
**Given** `helix.starters.security.enabled: false` est défini explicitement dans la config
**Then** le starter security NE s'active PAS même si un `SecurityConfigurer` est présent (override explicite prioritaire sur l'auto-détection)
**And** l'auto-détection par markers est complémentaire à la détection existante via `go.mod` (Stories 7.1–7.4)
**And** le log de démarrage indique pour chaque starter : la raison d'activation (`go.mod`, `config key`, ou `component marker`)

---

## Epic 8: Sécurité

Un développeur peut sécuriser son API avec JWT, RBAC et une configuration de sécurité déclarative.

### Story 8.1: JWT — Génération, Validation & Refresh

En tant que **développeur utilisant Helix**,
Je veux générer et valider des tokens JWT sans écrire de code de cryptographie,
Afin de sécuriser mon API avec un standard industriel.

**Acceptance Criteria:**

**Given** `security.jwt.secret` configuré dans `application.yaml`
**When** `jwtService.Generate(claims)` est appelé
**Then** un token JWT signé est retourné avec expiration configurable (`security.jwt.expiry`)
**When** un token valide est fourni dans le header `Authorization: Bearer <token>`
**Then** le token est validé et les claims sont disponibles dans le `helix.Context`
**When** un token expiré ou invalide est fourni
**Then** la réponse est `401 Unauthorized` avec le détail de l'erreur
**When** `jwtService.Refresh(token)` est appelé avec un token encore valide
**Then** un nouveau token est retourné avec une expiration réinitialisée

### Story 8.2: RBAC — Guards de Rôles Déclaratifs

En tant que **développeur utilisant Helix**,
Je veux restreindre l'accès à mes routes par rôle via une directive,
Afin d'implémenter le contrôle d'accès sans logique dans chaque handler.

**Acceptance Criteria:**

**Given** une route annotée `//helix:guard role:admin`
**When** un utilisateur authentifié avec le rôle `user` accède à la route
**Then** la réponse est `403 Forbidden`
**When** un utilisateur avec le rôle `admin` accède à la route
**Then** la requête est transmise au handler
**And** les rôles sont lus depuis les claims JWT (`roles` claim)
**And** plusieurs rôles acceptés peuvent être spécifiés : `//helix:guard role:admin,moderator`

### Story 8.3: `helix.SecurityConfigurer` — Règles Globales

En tant que **développeur utilisant Helix**,
Je veux définir les règles de sécurité globales de mon API en un seul endroit,
Afin d'avoir une vue d'ensemble de la politique d'accès sans parcourir chaque controller.

**Acceptance Criteria:**

**Given** une struct avec embed `helix.SecurityConfigurer` et méthode `Configure(http helix.HttpSecurity)`
**When** l'application démarre
**Then** les règles définies sont appliquées à toutes les routes correspondantes
**And** `http.Route("/actuator/**").PermitAll()` rend les endpoints actuator publics
**And** `http.Route("/api/**").Authenticated()` exige un JWT valide sur toutes les routes `/api/*`
**And** `http.Route("/admin/**").HasRole("ADMIN")` restreint l'accès aux admins
**And** les règles du `SecurityConfigurer` ont priorité sur les guards déclaratifs en cas de conflit

---

## Epic 9: Scheduling

Un développeur peut planifier des tâches récurrentes de façon déclarative via une directive cron.

### Story 9.1: Scheduler — Interface & Adaptateur robfig/cron

En tant que **développeur utilisant Helix**,
Je veux un scheduler intégré sans configuration manuelle,
Afin que mes tâches planifiées soient gérées par le framework.

**Acceptance Criteria:**

**Given** le package `scheduler/` importé
**When** `scheduler.NewScheduler()` est créé
**Then** l'interface `Scheduler` expose `Register(job Job) error`, `Start()`, `Stop(ctx)`
**And** `scheduler/internal/cron_adapter.go` est le seul fichier qui importe `robfig/cron/v3`
**And** le scheduler démarre via `OnStart()` et s'arrête proprement via `OnStop()` (lifecycle intégré)
**And** les jobs en cours d'exécution lors du shutdown se terminent avant l'arrêt (graceful)

### Story 9.2: Tâches Planifiées Déclaratives (`//helix:scheduled`)

En tant que **développeur utilisant Helix**,
Je veux annoter mes méthodes avec une expression cron pour les planifier automatiquement,
Afin de ne pas enregistrer mes jobs manuellement.

**Acceptance Criteria:**

**Given** une méthode annotée `//helix:scheduled 0 0 * * *`
**When** l'application démarre
**Then** la méthode est enregistrée automatiquement comme job cron exécuté chaque jour à minuit
**Given** une méthode annotée `//helix:scheduled @every 1h`
**Then** la méthode est exécutée toutes les heures
**And** si la méthode retourne une `error`, elle est loggée avec le nom du job sans crasher le scheduler
**And** deux exécutions simultanées du même job sont empêchées par défaut (lock distributable opt-in)
**And** le starter scheduling s'active automatiquement si des méthodes `//helix:scheduled` sont détectées

---

## Epic 10: CLI & Génération de Code

Un développeur peut scaffolder un projet, générer des modules, gérer les migrations et construire du code compile-time depuis le terminal.

### Story 10.1: Moteur de Génération de Code — Scanner AST

En tant que **développeur utilisant Helix**,
Je veux que `helix generate` analyse mon code Go pour détecter les composants et directives,
Afin que la génération soit précise et basée sur le code réel.

**Acceptance Criteria:**

**Given** un projet Go avec des structs utilisant les embeds Helix et des directives `//helix:*`
**When** `helix generate` est exécuté
**Then** le scanner parse tous les packages Go via `go/ast` et `go/types`
**And** les structs avec embeds `helix.Service`, `helix.Controller`, `helix.Repository`, `helix.Component` sont détectés
**And** les directives `//helix:route`, `//helix:transactional`, `//helix:scheduled`, `//helix:guard` sont détectées
**And** les interfaces taguées `query:"auto"` sont détectées
**And** le scanner signale une erreur lisible pour chaque directive malformée

### Story 10.2: DI Codegen Compile-time (`helix generate wire`)

En tant que **développeur utilisant Helix**,
Je veux générer le câblage DI à la compilation pour éliminer la reflection en production,
Afin d'obtenir des performances maximales et un code de wiring auditable.

**Acceptance Criteria:**

**Given** un projet avec composants Helix détectés par le scanner
**When** `helix generate wire` est exécuté
**Then** un fichier `helix_wire_gen.go` est généré avec tous les `Register()` et `Resolve()` explicites
**And** le fichier généré commence par `// Code generated by helix generate. DO NOT EDIT.`
**And** le fichier généré ne contient aucun import `reflect`
**And** `helix.Run(App{Mode: helix.ModeWire})` utilise ce fichier au lieu du ReflectResolver
**And** le même code applicatif fonctionne en mode Wire et en mode Reflect sans modification

### Story 10.3: Scaffold Projet & Modules (`helix new`, `helix generate module`)

En tant que **développeur utilisant Helix**,
Je veux scaffolder un nouveau projet ou module depuis le terminal,
Afin de démarrer sans copier-coller de boilerplate.

**Acceptance Criteria:**

**Given** la commande `helix new app my-service`
**When** exécutée dans un répertoire vide
**Then** un projet Helix complet est créé : `go.mod`, `main.go`, `config/application.yaml`, structure de dossiers
**And** `go build ./...` passe sans erreur sur le projet généré
**Given** la commande `helix generate module user` dans un projet existant
**Then** les fichiers `users/service.go`, `users/repository.go`, `users/controller.go` sont générés avec les embeds Helix
**And** les fichiers générés compilent sans erreur et respectent les conventions de nommage Helix

### Story 10.4: Contextes de Domaine DDD-light (`helix generate context`)

En tant que **développeur utilisant Helix**,
Je veux scaffolder un contexte de domaine complet avec son API publique,
Afin d'organiser mon code par domaine métier plutôt que par couche technique.

**Acceptance Criteria:**

**Given** la commande `helix generate context accounts`
**When** exécutée dans un projet Helix
**Then** le dossier `accounts/` est créé avec `service.go`, `repository.go`, `controller.go`, `api.go`
**And** `accounts/api.go` expose les fonctions publiques du contexte (`CreateUser`, `GetUser`)
**And** les autres packages appellent le contexte via `accounts.CreateUser(attrs)` sans importer ses internals
**And** les fichiers générés compilent et respectent les conventions Helix

### Story 10.5: Migrations DB CLI (`helix db migrate`)

En tant que **développeur utilisant Helix**,
Je veux gérer mes migrations de base de données depuis le terminal,
Afin de versionner l'évolution de mon schéma sans dépendance externe.

**Acceptance Criteria:**

**Given** la commande `helix db migrate create add_users_table`
**When** exécutée
**Then** un fichier de migration versionné est créé dans `db/migrations/` au format `{timestamp}_{name}.go`
**Given** `helix db migrate up`
**Then** toutes les migrations non appliquées sont exécutées dans l'ordre chronologique
**And** les migrations appliquées sont tracées dans une table `helix_migrations`
**Given** `helix db migrate down`
**Then** la dernière migration appliquée est rollbackée
**Given** `helix db migrate status`
**Then** la liste des migrations avec leur statut (applied/pending) est affichée

### Story 10.6: `helix run` & `helix build`

En tant que **développeur utilisant Helix**,
Je veux des commandes de développement et de build intégrées au CLI,
Afin d'avoir un workflow de développement fluide sans configurer de Makefile.

**Acceptance Criteria:**

**Given** la commande `helix run`
**When** exécutée dans un projet Helix
**Then** l'application démarre avec hot-reload (redémarrage automatique si un fichier `.go` change)
**And** `helix generate` est automatiquement exécuté avant chaque redémarrage
**Given** la commande `helix build`
**Then** `helix generate` est exécuté, puis `go build -o bin/app ./cmd/...`
**And** le binaire produit est statique (CGO_ENABLED=0) par défaut
**And** `helix build --docker` génère également un `Dockerfile` minimal multi-stage

---

## Epic 11: Documentation & Guides Développeur

Un développeur peut apprendre et maîtriser Helix en moins de 30 minutes grâce à une documentation complète et des guides pratiques.

### Story 11.1: README enrichi — Quick Start < 30 min

En tant que **développeur découvrant Helix**,
Je veux un README complet avec un exemple fonctionnel dès les premières lignes,
Afin de comprendre la valeur du framework et démarrer sans consulter d'autre documentation.

**Acceptance Criteria:**

**Given** un développeur qui consulte le README pour la première fois
**When** il lit la section Quick Start
**Then** il peut créer une API CRUD complète (users) en suivant les étapes en moins de 30 minutes
**And** le README présente les fonctionnalités clés avec des snippets de code concis
**And** les badges CI, couverture et Go Report Card sont affichés et fonctionnels
**And** la section Installation couvre `go get` et les prérequis Go 1.21+
**And** des liens vers les guides détaillés dans `docs/` sont présents

### Story 11.2: Guide DI Container & Configuration

En tant que **développeur utilisant Helix**,
Je veux un guide détaillé sur le container DI et le système de configuration,
Afin de comprendre les concepts fondamentaux du framework.

**Acceptance Criteria:**

**Given** le fichier `docs/di-and-config.md`
**When** un développeur le lit
**Then** le guide explique `helix.Service`, `helix.Controller`, `helix.Repository`, `helix.Component`
**And** les tags `inject:"true"` et `value:"key"` sont documentés avec des exemples
**And** les scopes Singleton et Prototype sont expliqués
**And** la chaîne de priorité config (ENV > profil YAML > application.yaml > DEFAULT) est documentée
**And** les profils (`HELIX_PROFILES_ACTIVE`) et le rechargement dynamique (SIGHUP) sont couverts

### Story 11.3: Guide Couche HTTP — Routing, Guards & Extracteurs

En tant que **développeur utilisant Helix**,
Je veux un guide complet sur la couche HTTP déclarative,
Afin d'exposer une API REST sans boilerplate.

**Acceptance Criteria:**

**Given** le fichier `docs/http-layer.md`
**When** un développeur le lit
**Then** les conventions de nommage (Index/Show/Create/Update/Delete) sont documentées avec des exemples
**And** les directives `//helix:route`, `//helix:guard`, `//helix:interceptor` sont expliquées
**And** les extracteurs typés (query params, body JSON, validation) sont documentés
**And** le mapping automatique des types de retour vers HTTP status est expliqué
**And** l'error handler centralisé (`//helix:handles`) est couvert

### Story 11.4: Guide Data Layer & Repository Pattern

En tant que **développeur utilisant Helix**,
Je veux un guide sur l'accès aux données avec le pattern Repository,
Afin de persister mes entités sans écrire de SQL.

**Acceptance Criteria:**

**Given** le fichier `docs/data-layer.md`
**When** un développeur le lit
**Then** l'interface `Repository[T, ID]` est documentée avec tous ses méthodes
**And** l'utilisation de `data/gorm.NewRepository` est expliquée avec des exemples
**And** le tag `query:"auto"` et les conventions de nommage de méthodes sont documentés
**And** la directive `//helix:transactional` est expliquée avec un exemple complet
**And** la pagination via `Paginate(page, size)` est documentée

### Story 11.5: Guide Sécurité, Observabilité & Scheduling

En tant que **développeur utilisant Helix**,
Je veux un guide sur les modules transversaux (sécurité, observabilité, scheduling),
Afin de sécuriser et monitorer mon application de production.

**Acceptance Criteria:**

**Given** le fichier `docs/security-observability-scheduling.md`
**When** un développeur le lit
**Then** la configuration JWT (`security.jwt.secret`, `security.jwt.expiry`) est documentée
**And** le RBAC déclaratif (`//helix:guard role:admin`) est expliqué avec des exemples
**And** `helix.SecurityConfigurer` est documenté avec un exemple de règles globales
**And** les endpoints `/actuator/health`, `/actuator/metrics`, `/actuator/info` sont décrits
**And** la directive `//helix:scheduled` avec expressions cron est documentée

---

## Epic 12: Exemples d'Applications

Un développeur peut partir d'un exemple concret et fonctionnel pour bootstrap son projet Helix.

### Story 12.1: Exemple CRUD API — Users (complet et fonctionnel)

En tant que **développeur découvrant Helix**,
Je veux un exemple d'application CRUD complète avec toutes les couches du framework,
Afin de voir comment les composants s'assemblent dans un projet réel.

**Acceptance Criteria:**

**Given** le répertoire `examples/crud-api/`
**When** `go run ./examples/crud-api` est exécuté
**Then** un serveur HTTP démarre sur le port 8080 sans erreur
**And** les endpoints `GET/POST/PUT/DELETE /users` sont fonctionnels
**And** l'exemple utilise `helix.Controller`, `helix.Service`, `helix.Repository`
**And** la configuration est chargée depuis `examples/crud-api/config/application.yaml`
**And** le README de l'exemple explique comment le lancer et le tester

### Story 12.2: Exemple avec Authentification JWT & RBAC

En tant que **développeur souhaitant sécuriser son API**,
Je veux un exemple complet avec authentification JWT et contrôle d'accès par rôle,
Afin d'avoir un point de départ concret pour la sécurisation de mon API.

**Acceptance Criteria:**

**Given** le répertoire `examples/secured-api/`
**When** `go run ./examples/secured-api` est exécuté
**Then** un endpoint `POST /auth/login` retourne un token JWT valide
**And** les endpoints protégés retournent `401` sans token valide
**And** les endpoints avec `//helix:guard role:admin` retournent `403` pour les non-admins
**And** `helix.SecurityConfigurer` est utilisé pour définir les règles globales
**And** un README explique le flux d'authentification

---

## Epic 13: Assainissement Technique (Dette)

Le framework est exempt de data races, de bugs de sécurité connus et de limitations majeures documentées dans deferred-work.md.

### Story 13.1: Sécurité — DSN credentials & bypass URL

En tant que **opérateur déployant Helix en production**,
Je veux que les credentials de base de données ne soient pas exposés dans les processus système et que les règles de sécurité ne puissent pas être contournées par des URLs encodées,
Afin de garantir la sécurité de l'application.

**Acceptance Criteria:**

**Given** la migration `helix db migrate up` est exécutée
**When** `ps aux` est inspecté pendant l'exécution
**Then** le DSN de la base de données n'est pas visible dans les arguments du sous-processus
**And** le DSN est transmis via variable d'environnement au sous-processus `go run`
**Given** une règle `SecurityConfigurer` bloquant `/api/**`
**When** une requête arrive sur `/api%2Fusers` (chemin URL-encodé)
**Then** la règle s'applique correctement et la requête est bloquée
**And** `matchesPattern` normalise les chemins avant comparaison
**Refs:** D-10.5-1, W2 [cli/internal/migrate/migrate.go, security/configurer.go]

### Story 13.2: Thread-safety du container DI & resolver

En tant que **développeur Helix**,
Je veux que le container DI soit sûr en concurrence,
Afin d'éviter les data races dans les applications qui résolvent des dépendances depuis plusieurs goroutines.

**Acceptance Criteria:**

**Given** un container avec plusieurs composants enregistrés
**When** `container.Resolve()` est appelé simultanément depuis N goroutines
**Then** aucune data race n'est détectée par `go test -race`
**And** les maps `registrations`, `singletons` et `graph.Edges` sont protégées par un mutex
**And** `Container.Resolve()` est protégé par un `sync.RWMutex` (read-lock pour la résolution, write-lock pour l'enregistrement)
**Refs:** core/reflect_resolver.go (sync absent), core/container.go, core/wire_resolver.go

### Story 13.3: Robustesse des starters — détection go.mod walk-up

En tant que **développeur utilisant Helix**,
Je veux que les starters web, data et scheduling s'activent correctement quel que soit le répertoire de travail du processus,
Afin que mon application fonctionne depuis n'importe quel CWD (déploiement, tests, CI).

**Acceptance Criteria:**

**Given** un binaire Helix démarré depuis `/var/app/` alors que `go.mod` est dans `/var/app/`
**Then** le starter web détecte bien `gofiber/fiber` dans `go.mod`
**Given** un binaire démarré depuis `/tmp/` (CWD ≠ racine du module)
**Then** le starter effectue un walk-up jusqu'à trouver le `go.mod` ou la racine du FS
**And** si aucun `go.mod` n'est trouvé, le starter logue un warning et reste inactif
**And** le comportement est identique pour les starters web, data et scheduling
**Refs:** D-7.2-1, D-7.3, D-7.4-1, D-9.1-3 [starter/web/starter.go, starter/data/starter.go, starter/scheduling/starter.go]

### Story 13.4: Compatibilité binaires déployés — suppression AST runtime

En tant que **développeur déployant Helix en production**,
Je veux que les directives `//helix:route` et `//helix:handles` fonctionnent sans sources Go sur le serveur,
Afin que mon application déployée en binaire statique soit pleinement opérationnelle.

**Acceptance Criteria:**

**Given** un binaire compilé avec `go build -trimpath` sans sources Go sur le serveur
**When** le serveur démarre
**Then** toutes les routes déclarées via `//helix:route` sont bien enregistrées
**And** tous les error handlers déclarés via `//helix:handles` sont bien enregistrés
**And** le mécanisme remplace `parser.ParseFile` par une registration explicite au démarrage (ex: `RegisterRoute(method, path, handler)` généré par `helix generate`)
**Refs:** D1/D2 [web/router.go:controllerRouteDirectives], [web/server.go:RegisterErrorHandler]

### Story 13.5: Cache interceptor production-grade

En tant que **développeur utilisant `//helix:interceptor cache`**,
Je veux un interceptor de cache robuste sans stampede ni fuite mémoire,
Afin de l'utiliser en production sans risque de surcharge ou de consommation mémoire illimitée.

**Acceptance Criteria:**

**Given** N requêtes simultanées sur un cache froid pour la même clé
**When** le handler est appelé
**Then** un seul appel au handler est effectué (single-flight pattern)
**And** les autres requêtes attendent et reçoivent la réponse mise en cache
**Given** le cache est en production depuis 1h avec des milliers de clés
**Then** le cache ne dépasse pas la taille maximale configurée (`cache:5m:max=1000`)
**And** les entrées expirées sont évincées proactivement par un goroutine de sweep
**Refs:** D-3.7-3, D-3.7-4, D-3.7-5 [web/cache_interceptor.go]

### Story 13.6: Qualité & UX développeur — validation, routing & erreurs

En tant que **développeur utilisant Helix**,
Je veux des messages d'erreur clairs et une UX de validation améliorée,
Afin de diagnostiquer rapidement les problèmes dans mon code.

**Acceptance Criteria:**

**Given** un handler avec un body ayant plusieurs champs invalides
**When** la requête arrive
**Then** toutes les erreurs de validation sont retournées (pas seulement la première)
**And** le format `{"errors": [{"field": "email", "msg": "required"}, ...]}` est retourné
**Given** un `UserHTTPController` enregistré
**Then** la route générée est `/user-https` → correction : les acronymes terminaux sont gérés correctement
**And** un préfixe de route override est possible via tag `helix:"route:/v1/users"` sur la struct
**Given** un `controller.Register` échoue
**Then** le message d'erreur indique le type du controller et la raison précise (pas juste `ErrInvalidController`)
**Refs:** D-3.4-4, D-3.2-1/2/3/5/6, D-3.5-1 [web/binding.go, web/router.go]

---

## Epic 14: Dette Technique v2

Le framework est robuste face aux edge cases de production : panics, structs embarquées, ScopePrototype correct, RBAC sécurisé, migrations sérialisées et goroutine leaks absents.
**FRs couverts :** NFR1, NFR2, NFR4 (dette restante post-v1.0)
**Phase PRD :** Phase 4 (Post-MVP)

### Story 14.1: Panic Recovery & HTTP Robustesse

En tant que **développeur utilisant Helix en production**,
Je veux que mon serveur HTTP survive aux panics dans les handlers et sérialise correctement toutes les erreurs,
Afin d'éviter des crashes de serveur et des réponses JSON silencieusement incorrectes.

**Acceptance Criteria:**

**Given** un handler qui panique (`panic("unexpected nil")`)
**When** une requête arrive sur ce handler
**Then** le serveur continue de répondre aux autres requêtes (recovery middleware Fiber)
**And** la réponse est `500 Internal Server Error` avec un body JSON structuré
**Given** un handler retournant `(error, nil)` via slot `any`
**Then** l'erreur est loggée et la réponse est `500` (pas `{}` silencieux)
**Given** une sérialisation JSON du résultat échoue
**Then** l'erreur est loggée avec le type du handler et la réponse est `500`
**Given** une requête POST sans `Content-Type: application/json`
**Then** Helix retourne `400 Bad Request` avec un message explicite avant de tenter le décodage
**Refs:** D-3.5-2, D-3.5-3, D-3.5-4, D-3.4-2 [web/internal/fiber_adapter.go, web/binding.go]

### Story 14.2: Binding Avancé — Structs Embarquées & Body Null

En tant que **développeur utilisant Helix**,
Je veux que le binding JSON visite les structs embarquées et gère le body null correctement,
Afin que mes DTOs complexes soient toujours correctement désérialisés.

**Acceptance Criteria:**

**Given** un handler avec un body struct contenant un champ embarqué (`type Req struct { Base; Name string }`)
**When** une requête arrive avec `{"name": "test", "base_field": "val"}`
**Then** les champs de `Base` sont correctement bindés (anonymous fields visités)
**Given** une requête POST avec body `null`
**Then** Helix retourne `400 Bad Request` (null ne bypass pas le guard "empty body")
**Given** une struct bindée avec `DisallowUnknownFields()` actif
**Then** un champ inconnu retourne `400` avec le nom du champ inconnu dans l'erreur
**And** un mécanisme opt-out `helix:"allow-unknown"` permet de désactiver ce comportement
**Refs:** D-3.4-3, D-3.4-5, D-3.4-1 [web/binding.go, web/internal/binding_plan.go]

### Story 14.3: Container DI — ScopePrototype Correct & Cycles

En tant que **développeur utilisant Helix**,
Je veux que `ScopePrototype` retourne de vraies nouvelles instances et que les cycles DI ne causent pas de stack overflow,
Afin d'avoir un container DI correct et sûr.

**Acceptance Criteria:**

**Given** un composant enregistré avec `ScopePrototype`
**When** `container.Resolve()` est appelé deux fois
**Then** deux pointeurs différents sont retournés (via `reflect.New`, pas le pointeur enregistré)
**Given** une dépendance cyclique `A → B → A`
**When** `container.Resolve()` est appelé
**Then** l'erreur `ErrCyclicDep` est retournée immédiatement (pas de récursion infinie ni stack overflow)
**Given** `container.Graph()` est appelé sur un container vide
**Then** `Graph().Edges` est une map initialisée non-nil (pas de panic si le caller écrit dedans)
**Given** `core.ComponentRegistration{Scope: ScopePrototype, Lazy: true}`
**Then** `container.Register()` retourne une erreur explicite (combinaison invalide rejetée au register)
**Refs:** D-1.3-Df3, D-1.3-Df1, D-1.2-D1, D-1.5-Df3 [core/reflect_resolver.go, core/container.go]

### Story 14.4: Sécurité — RBAC Case-Insensitive & Injection SQL

En tant que **développeur sécurisant son API avec Helix**,
Je veux que les rôles RBAC soient comparés sans sensibilité à la casse et que les filtres repository ne soient pas vulnérables à l'injection,
Afin d'éviter des bypasses de sécurité silencieux.

**Acceptance Criteria:**

**Given** un utilisateur avec le rôle `ADMIN` (majuscules) et une route `//helix:guard role:admin`
**When** la requête arrive
**Then** le guard autorise l'accès (comparaison case-insensitive via `strings.EqualFold`)
**And** le contrat est documenté dans le godoc de `RoleGuard`
**Given** un `Condition{Field: "name; DROP TABLE users;--"}` passé à un repository GORM
**When** `FindWhere(condition)` est exécuté
**Then** le nom de colonne est validé contre un pattern `^[a-zA-Z_][a-zA-Z0-9_]*$` avant usage
**And** un nom de colonne invalide retourne une `ErrInvalidCondition` (pas une requête SQL potentiellement dangereuse)
**Refs:** D-8.2-1, D-4.1-1, D-3.5-6 [security/rbac.go, data/gorm/repository.go]

### Story 14.5: Starters — Erreurs Propagées & Idempotence

En tant que **développeur utilisant Helix**,
Je veux que les erreurs d'enregistrement DI dans les starters soient propagées et que `Configure()` soit idempotent,
Afin de détecter les problèmes de configuration au démarrage plutôt qu'en production.

**Acceptance Criteria:**

**Given** un starter dont `container.Register(component)` échoue
**When** `Configure(container)` est appelé
**Then** l'erreur est propagée et `helix.Run()` échoue avec un message identifiant le starter et le composant
**And** ce pattern s'applique à TOUS les starters (web, data, observability, security, scheduling)
**Given** `Configure(container)` est appelé deux fois sur le même starter
**Then** aucun lifecycle dupliqué n'est créé (idempotence garantie)
**Given** `server.port: "99999"` dans la config
**Then** le starter web rejette le port au démarrage avec `ErrInvalidPort`
**Refs:** D-7.4-2, D-9.1-4, D-7.4-3, D-7.2-2 [starter/web/starter.go, starter/scheduling/starter.go, starter/observability/starter.go]

### Story 14.6: Repository — Pagination Sécurisée & FindAll Bornée

En tant que **développeur utilisant Helix**,
Je veux que la pagination rejette les valeurs invalides et que `FindAll()` supporte une borne maximale,
Afin d'éviter des charges mémoire non bornées en production.

**Acceptance Criteria:**

**Given** `repo.Paginate(-1, 0)` est appelé
**When** la requête est exécutée
**Then** une `ErrInvalidPagination` est retournée (page ≥ 1, size ≥ 1)
**Given** `repo.FindAll()` est appelé sur une table avec 1 million de lignes
**Then** par défaut, `FindAll()` retourne au maximum 1000 enregistrements avec un warning loggé
**And** une option `WithoutLimit()` permet de récupérer tous les enregistrements explicitement
**Given** `OperatorContains` avec `value = "50% off"` (contient `%`)
**Then** le `%` est échappé avant d'être interpolé dans le LIKE SQL
**Refs:** D-4.1-3, D-4.1-5, D-4.1-2, D-4.1-7 [data/repository.go, data/gorm/repository.go]

### Story 14.7: CLI Migrations — Concurrence & Annulation

En tant que **développeur utilisant Helix**,
Je veux que les migrations soient sérialisées et que l'annulation de contexte soit gérée proprement,
Afin d'éviter les états inconsistants en base lors d'exécutions concurrentes ou d'interruptions.

**Acceptance Criteria:**

**Given** deux processus exécutant `helix db migrate up` simultanément
**When** les deux atteignent la même migration non appliquée
**Then** un seul exécute la migration (advisory lock DB ou mécanisme de sérialisation)
**Given** le contexte est annulé après k migrations appliquées
**When** la migration k+1 démarre
**Then** `Up()` retourne immédiatement avec une erreur listant les k migrations déjà appliquées
**Given** `CGO_ENABLED=0` et un driver SQLite3 requis
**When** `helix db migrate up` est exécuté
**Then** le message d'erreur explique que `go-sqlite3` requiert CGo et suggère une alternative
**Refs:** D-10.5-2, D-10.5-5, D-10.5-3 [cli/internal/migrate/migrate.go]

### Story 14.8: Lifecycle — Goroutine Leaks & OnStop Garanti

En tant que **développeur utilisant Helix**,
Je veux qu'aucune goroutine ne soit laissée ouverte lors du shutdown et qu'`OnStop` soit toujours appelé même si `OnStart` échoue,
Afin d'éviter les memory leaks et les ressources non libérées.

**Acceptance Criteria:**

**Given** un composant dont `OnStop()` dépasse le timeout configuré
**When** le shutdown est déclenché
**Then** la goroutine de `OnStop()` est annulée via contexte (pas de goroutine leak)
**Given** un composant dont `OnStart()` retourne une erreur
**When** `helix.Run()` gère l'erreur
**Then** `OnStop()` est appelé sur tous les composants déjà démarrés (cleanup garanti)
**Given** `scheduler.Stop(ctx)` puis `lifecycle.OnStop()` sont appelés en séquence
**Then** le deuxième `cron.Stop()` est sans effet (idempotence documentée et testée)
**Refs:** D-1.7-Df4, D-7.4-4, D-9.1-2 [core/lifecycle.go, starter/scheduling/starter.go]

### Story 14.9: MockBean & TestApp — Edge Cases Multi-Interface

En tant que **développeur testant son application Helix**,
Je veux que `MockBean[T]` gère correctement les composants multi-interfaces,
Afin que mes tests ne tombent pas en erreur à cause d'ambiguïtés de résolution.

**Acceptance Criteria:**

**Given** un composant `SmtpMailer` implémentant `Mailer` et `HealthIndicator`
**When** `helix.MockBean[Mailer](mockMailer)` est utilisé dans un test
**Then** seule l'interface `Mailer` est remplacée — `HealthIndicator` reste résolu normalement
**Given** un mock implémentant des interfaces supplémentaires au-delà de la cible `T`
**When** `NewTestApp` démarre le container
**Then** aucun `ErrUnresolvable` "multiple registrations" n'est retourné
**Given** un `ComponentRegistration` passé via `TestContainerOptions`
**Then** `isReplacedComponent` le reconnaît correctement (pas de bypass du remplacement)
**Refs:** D-5.2-1, D-5.2-2, D-5.2-3 [testutil/testapp.go, core/reflect_resolver.go]

### Story 14.10: Guards, Context & Nettoyage Sentinelles

En tant que **développeur utilisant Helix**,
Je veux que les guards n'interrompent pas la chaîne d'interceptors et que les sentinelles mortes soient supprimées,
Afin d'avoir un comportement prédictible et un code base sans code mort.

**Acceptance Criteria:**

**Given** une route avec `//helix:interceptor log` et `//helix:guard authenticated` enchaînés
**When** une requête non authentifiée arrive
**Then** l'interceptor `log` est toujours exécuté (avant le guard) pour les requêtes rejetées
**Given** `web.Context` exposé dans l'API publique
**When** un développeur appelle `ctx.Method()` et `ctx.OriginalURL()`
**Then** ces méthodes sont disponibles (ajout non-cassant à l'interface `web.Context`)
**Given** `ErrJobNotFound` dans le package `scheduler`
**Then** le sentinel est soit utilisé dans l'implémentation, soit supprimé (zéro code mort)
**Given** `container.Register(component)` appelé deux fois avec le même type
**Then** les singletons précédemment résolus qui dépendent de ce type sont invalidés (ou l'erreur est documentée)
**Refs:** D-3.7-1, D-3.7-2, D-9.1-1, D-1.5-Df4 [web/guard.go, web/context.go, scheduler/scheduler.go]
