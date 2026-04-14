---
stepsCompleted: [1, 2, 3, 4]
status: 'complete'
completedAt: '2026-04-14'
inputDocuments:
  - '_bmad-output/product-development/PRD.md'
  - '_bmad-output/planning-artifacts/architecture.md'
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

## Epic List

### Epic 1: Application Bootstrap & DI Container
Un développeur peut créer une application Helix avec injection de dépendances automatique en une ligne de code.
**FRs couverts :** FR1, FR2, FR4, FR5, FR17
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
Un développeur peut démarrer une application Helix sans aucune configuration explicite — les starters s'activent automatiquement.
**FRs couverts :** FR27
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
