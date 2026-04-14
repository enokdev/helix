# PRD — Helix : Framework Go inspiré de Spring Boot

**Version** : 1.0  
**Date** : 2026-04-14  
**Statut** : Draft  

---

## 1. Problème

Les développeurs Go qui construisent des APIs et des microservices en production font face à un problème structurel : **l'écosystème Go ne propose pas de framework d'application complet et opinionné**.

Les solutions actuelles imposent un choix pénible :

- **Utiliser des frameworks HTTP légers** (`chi`, `echo`, `gin`, `fiber`) : rapides, mais sans DI, sans gestion du cycle de vie, sans configuration centralisée, sans observabilité intégrée. Chaque équipe réinvente la même plomberie.
- **Utiliser `uber-go/fx` ou `google/wire`** : puissants pour la DI, mais sans layer HTTP intégré, sans repository pattern, sans auto-configuration. L'intégration est manuelle et fragmentée.
- **Porter des habitudes Spring Boot en Go** : impossible sans un framework qui parle le même langage conceptuel (starters, auto-config, actuator, lifecycle).

Le résultat : les équipes Go perdent du temps à assembler des solutions maison, souvent incohérentes d'un projet à l'autre, difficiles à maintenir et à onboarder.

---

## 2. Vision produit

Helix est un framework backend Go **complet, modulaire et opinionné**, qui offre aux développeurs les mêmes garanties structurelles que Spring Boot — DI/IoC, configuration centralisée, auto-configuration, observabilité — **sans sacrifier les principes Go** : performances, typage fort, clarté, pas de magie cachée.

> "Ce que Spring Boot est à Java, Helix l'est à Go — mais idiomatique."

---

## 3. Cibles

| Cible | Profil |
|---|---|
| Développeurs Go expérimentés | Connaissent Go, viennent d'un projet sans framework structurant |
| Équipes Java/Spring migrantes | Migrent vers Go et veulent retrouver leurs repères conceptuels |
| Équipes microservices | Buildent plusieurs services, veulent une base cohérente |
| Startups / SaaS | Veulent démarrer vite avec une architecture production-ready |

---

## 4. Différenciation concurrentielle

| Outil | Ce qu'il fait bien | Ce qui manque |
|---|---|---|
| `uber-go/fx` | DI runtime puissant | Pas de HTTP, pas de config, pas d'auto-config |
| `google/wire` | DI compile-time | Pas d'intégration applicative |
| `buffalo` | Full-stack opinionné | Orienté monolithe web, pas microservices |
| `echo` / `gin` / `chi` | HTTP performant | Pas de DI, pas de lifecycle, pas d'observabilité |
| Spring Boot | Framework complet | JVM, démarrage lent, performance limitée |

**Positionnement Helix** : premier framework Go à combiner DI compile-time, HTTP performant (Fiber), auto-configuration par module, et observabilité production-ready dans un seul outil cohérent.

---

## 5. Fonctionnalités

### 5.1 Dependency Injection (DI) / IoC

**Mode principal — Code generation (compile-time)**

- Wiring déclaré via struct tags : `inject:"true"`
- Résolution des dépendances à la compilation (inspiré de Wire)
- Typage fort, zéro reflection au runtime
- Détection des cycles de dépendance à la compilation

**Mode avancé — Reflection (opt-in)**

- Container dynamique pour les cas d'usage avancés
- Les deux modes sont **mutuellement exclusifs par module** : un module déclare son mode, pas de mélange implicite
- En cas d'ambiguïté, le mode codegen prend toujours la priorité

**API publique**

```go
// Déclaration
type UserService struct {
    Repo UserRepository `inject:"true"`
}

// Wiring
container := helix.NewContainer()
container.Register(UserRepositoryImpl{})
container.Resolve(&UserService{})
```

**Scopes supportés** : Singleton, Prototype  
**Lazy loading** : oui, configurable par composant

---

### 5.2 Configuration centralisée

**Formats supportés** : YAML (principal), JSON, TOML, variables d'environnement

**Priorité de résolution** :
```
ENV > YAML > DEFAULT
```

**Mapping Go**

```go
type ServerConfig struct {
    Port int `mapstructure:"port"`
    Host string `mapstructure:"host"`
}
```

**Rechargement dynamique (optionnel)**

- Déclenché par `SIGHUP` ou polling filesystem (intervalle configurable)
- Les composants singleton déjà injectés reçoivent un callback `OnConfigReload()` s'ils implémentent l'interface `ConfigReloadable`
- Aucune garantie de cohérence pour les composants Prototype — comportement documenté explicitement

**Namespaces** : chaque module déclare son propre namespace de configuration pour éviter les collisions.

---

### 5.3 Auto-configuration & Starters

**Principe** : convention over configuration — un starter est actif si et seulement si sa dépendance est présente dans `go.mod` et sa configuration minimale fournie.

**Mécanisme de détection** : à l'initialisation du container, Helix inspecte les modules Go enregistrés et active les starters correspondants. La détection est déterministe et loggée au démarrage.

**Starters prévus**

| Starter | Activation | Responsabilité |
|---|---|---|
| `web` | `fiber` présent dans go.mod | Auto-configure le serveur HTTP Fiber |
| `data` | driver DB présent + config `database.*` | Initialise la connexion et les repositories |
| `security` | config `security.*` présente | Active JWT, RBAC, middleware d'auth |
| `config` | toujours actif | Charge YAML/ENV au démarrage |
| `observability` | config `observability.*` présente | Active Prometheus, slog, OpenTelemetry |

---

### 5.4 Data Layer — Repository Pattern

**Interface générique**

```go
type Repository[T any, ID any] interface {
    FindAll() ([]T, error)
    FindByID(id ID) (*T, error)
    FindWhere(filter Filter) ([]T, error)
    Save(entity *T) error
    Delete(id ID) error
    Paginate(page, size int) (Page[T], error)
    WithTransaction(tx Transaction) Repository[T, ID]
}
```

Le type `ID` est paramétré pour supporter `int`, `int64`, `string`, `uuid.UUID`, ou tout type comparable.

**Implémentations prévues**

- GORM (intégration Phase 1)
- Ent (Phase 2)
- sqlc (Phase 3)

**Fonctionnalités transversales** : pagination native, transactions explicites, query builder optionnel.

---

### 5.5 Web Layer (Fiber)

- Routage automatique basé sur conventions (contrôleurs annotés)
- Injection des dépendances dans les handlers via le container
- Validation des requêtes intégrée
- Middleware fournis : logging, recovery, auth, rate limiting, CORS

```go
app.Get("/users", userController.GetAll)
```

---

### 5.6 Cycle de vie applicatif

```go
type Lifecycle interface {
    OnStart() error
    OnStop() error
}
```

- **Graceful shutdown** : le container attend la fin des requêtes en cours avant d'appeler `OnStop()` sur chaque composant, dans l'ordre inverse d'initialisation
- **Timeout de shutdown** configurable (défaut : 30s)
- **Ordre d'initialisation** : déterministe, basé sur le graphe de dépendances

---

### 5.7 Observabilité

| Endpoint | Description |
|---|---|
| `GET /health` | État de l'application et de ses dépendances |
| `GET /metrics` | Métriques Prometheus |
| `GET /info` | Version, build info, configuration active |

- **Logging** : `slog` (Go 1.21+), JSON structuré, niveau configurable par namespace
- **Métriques** : Prometheus natif, avec extension OpenTelemetry optionnelle
- **Tracing** : OpenTelemetry (opt-in via starter observability)

---

### 5.8 Sécurité

- JWT : génération, validation, refresh
- RBAC : décorateur `@RequireRole("admin")` via struct tag sur les handlers
- Middleware configurable par route ou globalement

---

### 5.9 CLI

```bash
helix new app <nom>          # scaffold projet complet
helix generate module <nom>  # génère un module (controller, service, repository)
helix run                    # démarre l'application avec hot reload
helix build                  # build statique
```

---

## 6. Exigences techniques non fonctionnelles

| Exigence | Cible |
|---|---|
| Version Go minimale | Go 1.21 (requis pour `slog`) |
| Temps de démarrage | < 100ms (application standard, sans cold start DB) |
| Latence P99 | < 10ms pour les endpoints sans traitement métier |
| Taille binaire | < 20 MB pour une application standard |
| Dépendances externes | Minimisées — chaque starter est une dépendance optionnelle |

---

## 7. Critères de succès mesurables

| KPI | Cible | Méthode de mesure |
|---|---|---|
| Temps de démarrage | < 100ms | Benchmark automatisé dans la CI |
| Latence P99 endpoint `/health` | < 5ms | Test de charge avec `k6` |
| Couverture de tests | > 80% sur `core/` | `go test -cover` |
| Onboarding time | < 30 min pour créer une API CRUD complète | Test utilisateur avec 5 développeurs Go |
| Score DX (enquête) | > 4/5 | Feedback communautaire après beta |

---

## 8. Risques

| Risque | Probabilité | Impact | Mitigation |
|---|---|---|---|
| Adoption faible face à des projets mieux financés (`fx`, `wire`) | Élevée | Élevé | Différenciation claire sur l'expérience full-stack, documentation exemplaire |
| Mauvaise DX si le framework est "trop magique" | Moyenne | Élevé | Mode verbose de logging du wiring, erreurs explicites à la compilation |
| Debug difficile en mode reflection | Élevée | Moyen | Désactiver reflection par défaut, avertissement explicite dans la doc |
| Bus factor = 1 (projet solo) | Élevée | Élevé | Documenter les décisions d'architecture, encourager les contributions dès la Phase 1 |
| Breaking changes entre phases | Moyenne | Élevé | Versioning semver strict, deprecation warnings avant suppression |
| Dépendance à Fiber (si Fiber ralentit son développement) | Faible | Élevé | Abstraire le layer HTTP derrière une interface pour permettre un swap futur |

---

## 9. Roadmap

### Phase 1 — MVP (obligatoire avant Phase 2)

**Critère de sortie** : une API CRUD complète tourne avec DI, config YAML, et HTTP Fiber.

- Container DI (mode codegen uniquement)
- Loader de configuration YAML + ENV
- Intégration Fiber (starter web)
- Endpoint `/health` basique
- Tests unitaires sur `core/`

### Phase 2 — Structuration

**Critère de sortie** : un projet Helix est maintenable et extensible sans modifier le core.

- Repository pattern avec GORM
- Auto-configuration (starters `data`, `config`)
- Lifecycle hooks (OnStart / OnStop)
- Graceful shutdown

### Phase 3 — Production-ready

**Critère de sortie** : une application Helix peut être déployée en production avec observabilité complète.

- Observabilité complète (Prometheus, slog, OTel)
- Module sécurité (JWT, RBAC)
- CLI (`helix new`, `helix generate`, `helix run`)

### Phase 4 — Écosystème

- Modules cloud (Consul, circuit breaker)
- Système de plugins tiers
- Support Ent et sqlc

---

## 10. Hors périmètre

- ORM maison — Helix s'appuie sur des ORMs existants (GORM, Ent, sqlc)
- Support de protocoles non-HTTP (gRPC, WebSocket) en Phase 1-2
- Frontend / SSR — Helix est exclusivement backend
- Support multi-langage — Go uniquement

---

## 11. Questions ouvertes

1. Le mode reflection DI doit-il être dans le même binaire ou dans un module séparé (`helix-reflect`) ?
2. Faut-il un format de configuration unifié pour tous les starters, ou chaque starter définit-il son propre schéma ?
3. Quelle est la stratégie de migration recommandée pour les projets existants sous `echo` ou `gin` ?
