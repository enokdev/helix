# PRD — Helix : Framework Go inspiré de Spring Boot

**Version** : 2.1  
**Date** : 2026-04-14  
**Statut** : Draft  

---

## 1. Problème

Les développeurs Spring Boot qui migrent vers Go se retrouvent face à un écosystème fragmenté et sans repères. Les frameworks HTTP Go (`gin`, `echo`, `fiber`, `chi`) sont rapides mais ne proposent ni DI, ni gestion du cycle de vie, ni configuration centralisée, ni observabilité intégrée. Les outils DI (`uber-go/fx`, `google/wire`) sont puissants mais déconnectés de la couche HTTP et de la persistence.

**Conséquence :** chaque équipe réinvente la même plomberie. L'onboarding est coûteux. Les projets sont incohérents.

---

## 2. Vision

> Helix est le Spring Boot de Go — complet, opinionné, idiomatique.

Un développeur Spring Boot doit pouvoir ouvrir Helix et se sentir immédiatement chez lui. Les concepts mappent 1-pour-1. Les conventions sont identiques. Le framework "marche tout seul" par défaut.

---

## 3. Cibles

| Cible | Profil |
|---|---|
| Développeurs Spring Boot migrants | Connaissent Spring intimement, découvrent Go |
| Équipes microservices Go | Buildent plusieurs services, veulent une base cohérente |
| Startups / SaaS | Veulent démarrer vite avec une architecture production-ready |

---

## 4. Différenciation concurrentielle

| Outil | Ce qu'il fait bien | Ce qui manque |
|---|---|---|
| `uber-go/fx` | DI runtime puissant | Pas de HTTP, config, auto-config |
| `google/wire` | DI compile-time | Pas d'intégration applicative |
| `buffalo` | Full-stack opinionné | Orienté monolithe web, pas microservices |
| `echo` / `gin` / `chi` | HTTP performant | Pas de DI, lifecycle, observabilité |
| Spring Boot | Framework complet | JVM, démarrage lent |

**Positionnement Helix :** seul framework Go combinant DI, HTTP, Data, Config, Tests, Observabilité, CLI dans un outil cohérent orienté DX Spring Boot.

---

## 5. Fonctionnalités

### 5.1 Point d'entrée — `helix.Run()`

Le container est invisible. L'utilisateur ne le manipule jamais directement.

```go
func main() {
    helix.Run(App{})
}
```

Helix auto-scanne les composants enregistrés, résout les dépendances, démarre les starters actifs, lance le serveur HTTP.

---

### 5.2 Dependency Injection

**Mode par défaut — Code generation (compile-time)**

Conforme aux principes fondateurs du projet : explicite, sans magie, performant. `helix generate wire` analyse les struct tags et génère le câblage des dépendances au build time. Zéro reflection en production.

```go
type UserService struct {
    helix.Service                        // marqueur de composant
    Repo UserRepository `inject:"true"`  // injection câblée par codegen
    Port int            `value:"server.port"` // injection de config
}
```

Le fichier généré (`helix_wire_gen.go`) est versionnable, lisible, et ne dépend d'aucune reflection.

**Marqueurs de composant via embeds :**

| Spring | Helix |
|---|---|
| `@Service` | `helix.Service` (embed) |
| `@Controller` / `@RestController` | `helix.Controller` (embed) |
| `@Repository` | `helix.Repository` (embed) |
| `@Component` | `helix.Component` (embed) |

**Mode opt-in — Reflection (runtime)**

Pour les développeurs Spring Boot migrants qui préfèrent le démarrage sans étape de génération. Activé via `helix.ModeReflect` dans la config ou flag `--reflect`. Les deux modes partagent la même API (`inject:"true"`, `value:"key"`) — le mode peut être changé sans modifier les structs.

**Scopes :** Singleton (défaut), Prototype  
**Lazy loading :** oui, configurable par composant  
**Détection des cycles :** au démarrage, avec message d'erreur explicite

---

### 5.3 Configuration centralisée

Conventions identiques à Spring Boot — zéro réapprentissage.

**Structure :**
```
config/
  application.yaml          # config de base
  application-dev.yaml      # surcharge dev
  application-prod.yaml     # surcharge prod
```

**Activation de profil :**
```bash
HELIX_PROFILES_ACTIVE=prod go run .
```

**Priorité de résolution :**
```
ENV > YAML profil actif > application.yaml > DEFAULT
```

**Mapping Go :**
```go
type ServerConfig struct {
    Port int    `mapstructure:"port"`
    Host string `mapstructure:"host"`
}
```

**Rechargement dynamique (opt-in) :**
- Déclenché par `SIGHUP` ou polling filesystem (intervalle configurable)
- Les composants Singleton reçoivent un callback `OnConfigReload()` s'ils implémentent `helix.ConfigReloadable`
- Comportement Prototype documenté explicitement (pas de garantie de cohérence)

---

### 5.4 Web Layer — HTTP déclaratif

Fiber tourne sous le capot mais reste invisible. L'API publique est entièrement déclarative.

**Routing par convention (Rails-like) :**

```go
type UserController struct {
    helix.Controller
    Service UserService `inject:"true"`
}

// Méthodes conventionnelles → routes auto-générées
func (c *UserController) Index() {}    // GET /users
func (c *UserController) Show() {}     // GET /users/:id
func (c *UserController) Create() {}   // POST /users
func (c *UserController) Update() {}   // PUT /users/:id
func (c *UserController) Delete() {}   // DELETE /users/:id
```

**Routing custom via directives de commentaire :**
```go
//helix:route GET /users/search
func (c *UserController) Search() {}
```

Les directives `//helix:*` sont traitées par `helix generate` au build time et génèrent l'enregistrement des routes. En mode reflection, elles sont lues via `go/ast` au démarrage.

**Extracteurs typés automatiques :**
```go
type GetUsersParams struct {
    Page     int    `query:"page" default:"1"`
    PageSize int    `query:"page_size" default:"20" max:"100"`
    Search   string `query:"search"`
}

type CreateUserBody struct {
    Name  string `json:"name" validate:"required,min=2"`
    Email string `json:"email" validate:"required,email"`
}

// Helix parse, valide et injecte automatiquement
func (c *UserController) Index(params GetUsersParams) ([]User, error) {}
func (c *UserController) Create(body CreateUserBody) (*User, error) {}
```

**Mapping retour → HTTP status automatique :**

| Type retourné | Status HTTP |
|---|---|
| `(*T, nil)` sur GET | 200 + JSON |
| `(*T, nil)` sur POST | 201 + JSON |
| `(nil, helix.NotFoundError)` | 404 + JSON structuré |
| `(nil, helix.ValidationError)` | 400 + JSON structuré |
| `(nil, error)` générique | 500 + JSON structuré |

**Guards & Interceptors déclaratifs :**
```go
//helix:route GET /profile
//helix:guard authenticated
//helix:guard role:admin
//helix:interceptor cache:5m
func (c *UserController) GetProfile() error {}
```

---

### 5.5 Error Handling centralisé

```go
type AppErrorHandler struct {
    helix.ErrorHandler
}

//helix:handles ValidationError
func (h *AppErrorHandler) HandleValidation(err ValidationError) helix.HTTPResponse {
    return helix.BadRequest(err.Error())
}

//helix:handles NotFoundError
func (h *AppErrorHandler) HandleNotFound(err NotFoundError) helix.HTTPResponse {
    return helix.NotFound("Resource not found")
}
```

Les erreurs remontent automatiquement — les handlers ne gèrent que le happy path.

---

### 5.6 Data Layer — Repository Pattern

**Interface générique paramétrée sur le type d'ID :**

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

**Spring Data-like — génération automatique de requêtes :**

```go
type UserRepository interface {
    helix.Repository[User, int]

    FindByEmail(email string) (*User, error)                 `query:"auto"`
    FindByNameContaining(name string) ([]User, error)        `query:"auto"`
    FindByAgeGreaterThan(age int) ([]User, error)            `query:"auto"`
    FindByEmailAndAge(email string, age int) (*User, error)  `query:"auto"`
    FindAllOrderByCreatedAtDesc() ([]User, error)            `query:"auto"`
}
```

`helix generate` analyse les noms de méthodes taguées `query:"auto"` et génère l'implémentation SQL à la compilation.

**AOP Transactionnel :**

```go
//helix:transactional
func (s *UserService) CreateUser(user *User) error {
    // begin/commit/rollback automatique — généré par helix generate
    // rollback si error != nil
    return s.Repo.Save(user)
}
```

**Implémentations ORM supportées :**

| ORM | Phase |
|---|---|
| GORM | Phase 1 |
| Ent | Phase 2 |
| sqlc | Phase 3 |

---

### 5.7 Cycle de vie applicatif

```go
type Lifecycle interface {
    OnStart() error
    OnStop() error
}
```

- Graceful shutdown : attente de fin des requêtes en cours avant `OnStop()`
- Timeout de shutdown configurable (défaut : 30s)
- Ordre d'initialisation déterministe basé sur le graphe de dépendances
- Ordre de shutdown : inverse de l'initialisation

---

### 5.8 Observabilité

| Endpoint | Description |
|---|---|
| `GET /actuator/health` | État de l'application et ses dépendances |
| `GET /actuator/metrics` | Métriques Prometheus |
| `GET /actuator/info` | Version, build info, profil actif |

- **Logging** : `slog` (Go 1.21+), JSON structuré, niveau configurable par namespace
- **Métriques** : Prometheus natif + OpenTelemetry opt-in
- **Tracing** : OpenTelemetry via starter observability

---

### 5.9 Sécurité

```go
type SecurityConfig struct {
    helix.SecurityConfigurer
}

func (s *SecurityConfig) Configure(http helix.HttpSecurity) {
    http.
        Route("/actuator/**").PermitAll().
        Route("/api/**").Authenticated().
        Route("/admin/**").HasRole("ADMIN")
}
```

- JWT : génération, validation, refresh
- RBAC : guards déclaratifs sur les handlers
- Configuration centralisée des règles de sécurité

---

### 5.10 Tests

```go
func TestCreateUser(t *testing.T) {
    app := helix.NewTestApp(t,
        helix.MockBean[UserRepository](mockRepo),
        helix.MockBean[EmailService](mockEmail),
    )

    service := helix.GetBean[UserService](app)
    user, err := service.CreateUser(UserAttrs{Name: "Alice"})

    assert.NoError(t, err)
    assert.Equal(t, "Alice", user.Name)
}
```

- `helix.NewTestApp()` : démarre le container complet en test
- `helix.MockBean[T]()` : remplace un composant par un mock automatiquement
- `helix.GetBean[T]()` : récupère un composant du container de test

---

### 5.11 Tâches planifiées

```go
type ReportService struct {
    helix.Service
}

//helix:scheduled 0 0 * * *
func (s *ReportService) GenerateDailyReport() error {
    // cron job enregistré automatiquement au démarrage
}

//helix:scheduled @every 1h
func (s *ReportService) CleanupExpiredTokens() error {
    // exécuté toutes les heures
}
```

---

### 5.12 Contexts de domaine (DDD-light)

Organisation par domaine plutôt que par couche technique :

```bash
helix generate context accounts
# Génère : accounts/ avec service, repository, controller, config
```

```go
// API publique du context — les autres packages appellent ça
package accounts

func CreateUser(attrs UserAttrs) (*User, error) { ... }
func GetUser(id int) (*User, error) { ... }
```

---

### 5.13 Migrations DB

```bash
helix db migrate create add_users_table
helix db migrate up
helix db migrate down
helix db migrate status
```

Migrations versionnées en Go pur dans `db/migrations/`. Intégrées dans le CLI — pas de dépendance externe requise.

---

### 5.14 CLI

```bash
helix new app <nom>                  # scaffold projet complet
helix generate module <nom>          # génère module (controller, service, repository)
helix generate context <nom>         # génère context de domaine
helix generate repository <nom>      # génère repository avec query:"auto"
helix db migrate create <nom>        # crée un fichier de migration
helix db migrate up                  # applique les migrations
helix db migrate down                # rollback dernière migration
helix db migrate status              # état des migrations
helix run                            # démarre avec hot reload
helix build                          # build statique
```

---

### 5.15 Auto-configuration & Starters

| Starter | Activation | Responsabilité |
|---|---|---|
| `web` | `fiber` présent dans go.mod | Auto-configure serveur HTTP |
| `data` | driver DB + config `database.*` | Connexion + repositories |
| `security` | config `security.*` | JWT, RBAC, middleware d'auth |
| `config` | toujours actif | Charge YAML/ENV au démarrage |
| `observability` | config `observability.*` | Prometheus, slog, OTel |

---

## 6. Exigences non fonctionnelles

| Exigence | Cible |
|---|---|
| Version Go minimale | Go 1.21 (requis pour `slog`) |
| Temps de démarrage | < 100ms (application standard) |
| Latence P99 `/actuator/health` | < 5ms |
| Dépendances externes | Optionnelles — chaque starter est une dépendance conditionnelle |

---

## 7. Critères de succès mesurables

| KPI | Cible | Méthode |
|---|---|---|
| Temps de démarrage | < 100ms | Benchmark CI automatisé |
| Latence P99 `/actuator/health` | < 5ms | Test de charge `k6` |
| Couverture tests `core/` | > 80% | `go test -cover` |
| Onboarding time | < 30 min pour API CRUD complète | Test utilisateur (5 devs Spring) |
| Score DX | > 4/5 | Enquête beta |

---

## 8. Mapping Spring Boot → Helix

| Spring Boot | Helix |
|---|---|
| `@Service` | `helix.Service` (embed) |
| `@RestController` | `helix.Controller` (embed) |
| `@Repository` | `helix.Repository` (embed) |
| `@Autowired` | `inject:"true"` |
| `@Value("${key}")` | `value:"key"` |
| `@Transactional` | `//helix:transactional` |
| `@Scheduled(cron)` | `//helix:scheduled cron` |
| `@GetMapping("/path")` | `//helix:route GET /path` |
| `@PreAuthorize` | `//helix:guard role:admin` |
| `@ExceptionHandler` | `helix.ErrorHandler` + `//helix:handles Type` |
| `SpringApplication.run()` | `helix.Run(App{})` |
| `application-dev.yaml` | `application-dev.yaml` (identique) |
| `SPRING_PROFILES_ACTIVE` | `HELIX_PROFILES_ACTIVE` |
| `@SpringBootTest` + `@MockBean` | `helix.NewTestApp()` + `helix.MockBean[T]()` |
| Spring Data repositories | Interface + `query:"auto"` + codegen |

---

## 9. Risques

| Risque | Probabilité | Impact | Mitigation |
|---|---|---|---|
| Adoption faible face à `fx` et `wire` | Élevée | Élevé | Différenciation DX Spring, documentation exemplaire |
| DX "trop magique" pour la communauté Go-native | Moyenne | Élevé | Mode verbose de logging du wiring, erreurs explicites |
| Debug difficile en mode reflection | Élevée | Moyen | Logging détaillé du container au démarrage, flag `--debug-di` |
| Bus factor = 1 (projet solo) | Élevée | Élevé | Documentation des décisions d'architecture, contributions dès Phase 1 |
| Breaking changes entre phases | Moyenne | Élevé | Semver strict, deprecation warnings avant suppression |
| Dépendance à Fiber | Faible | Élevé | Layer HTTP abstrait derrière interface — swap possible |

---

## 10. Roadmap

### Phase 1 — MVP

**Critère de sortie :** API CRUD complète avec DI, config, HTTP, test.

- `helix.Run()` + container DI reflection
- Embeds de composants (`helix.Service`, `helix.Controller`, `helix.Repository`)
- Routing déclaratif (tags + convention `Index/Show/Create/Update/Delete`)
- Extracteurs typés + mapping retour → HTTP status
- Configuration YAML + profils (`HELIX_PROFILES_ACTIVE`)
- GORM + Repository générique
- `helix.NewTestApp()` + `helix.MockBean[T]()`
- Endpoint `/actuator/health`

### Phase 2 — Production-ready

**Critère de sortie :** Un service Helix peut être déployé en production.

- `transactional:"true"` AOP
- `query:"auto"` Spring Data-like + `helix generate repository`
- Error handler centralisé (`helix.ErrorHandler`)
- Lifecycle hooks complets + graceful shutdown
- Auto-configuration starters (web, data, config)
- `helix db migrate`
- Observabilité complète (Prometheus, slog, OTel)

### Phase 3 — Écosystème

**Critère de sortie :** DX complète, parité Spring Boot sur les features courantes.

- Guards & Interceptors déclaratifs
- `scheduled:"cron"` tâches planifiées
- Module sécurité (JWT, RBAC, `helix.SecurityConfigurer`)
- Contexts de domaine (`helix generate context`)
- CLI complet (`helix new`, `helix generate`, `helix run`)
- Support Ent + sqlc

### Phase 4 — Cloud & Plugin

- Modules cloud (Consul, circuit breaker)
- Système de plugins tiers
- Mode compile-time DI (opt-in)

---

## 11. Hors périmètre

- ORM maison — Helix s'appuie sur GORM, Ent, sqlc
- Support gRPC, WebSocket en Phase 1-2
- Frontend / SSR — backend uniquement
- Support multi-langage — Go uniquement

---

## 12. Questions ouvertes

1. Le mode reflection DI doit-il être dans le même binaire ou dans un module séparé (`helix-reflect`) ?
2. ~~Comment `helix generate` détecte-t-il les noms de méthodes pour `query:"auto"` ?~~ **Résolu** : `helix generate` utilise `go/ast` pour parser les interfaces taguées `query:"auto"` et génère l'implémentation SQL. Le parsing est fait au build time via `go generate`.
3. Les Contexts de domaine sont-ils encouragés par la documentation ou imposés par le generator ?
4. Quel est le comportement de `//helix:transactional` avec les modes Prototype (nouveau composant par appel) ?
