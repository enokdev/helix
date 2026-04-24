# Securite, observabilite et scheduling

Ce guide detaille les modules transversaux de Helix pour les applications de production : authentification JWT, RBAC, regles globales de securite, endpoints actuator, metriques Prometheus, logs structures, tracing OpenTelemetry et taches planifiees.

## Sommaire

- [Modele mental](#modele-mental)
- [Configuration JWT](#configuration-jwt)
- [Guards et RBAC](#guards-et-rbac)
- [Security configurer](#security-configurer)
- [Actuator](#actuator)
- [Metriques Prometheus](#metriques-prometheus)
- [Logging structure](#logging-structure)
- [Tracing OpenTelemetry](#tracing-opentelemetry)
- [Scheduling](#scheduling)
- [Starters](#starters)
- [Tests](#tests)
- [Erreurs frequentes](#erreurs-frequentes)

## Modele mental

Les modules transversaux Helix restent decouples :

- `security` fournit JWT, guards RBAC et regles globales.
- `observability` expose health/info/metrics, logs structures et tracing.
- `scheduler` enregistre et execute des jobs cron.
- `starter/*` branche ces modules automatiquement quand la configuration ou les dependances le permettent.

Les controllers continuent a utiliser `web.Context` et les directives `//helix:guard`. Les modules transversaux fournissent les guards, les endpoints et les composants de cycle de vie qui s'attachent au serveur ou au container.

## Configuration JWT

Le starter security lit les cles suivantes :

```yaml
helix:
  starters:
    security:
      enabled: true

security:
  jwt:
    secret: "change-me-in-production" # CRITIQUE : Utilisez un secret fort et unique
    expiry: "24h"
```

`helix.starters.security.enabled` force l'activation ou la desactivation du starter. Sans cette cle, le starter s'active si une cle top-level `security` existe. `security.jwt.secret` alimente le service JWT ; un secret vide ou par defaut en production compromet la securite. `security.jwt.expiry` est parse avec `time.ParseDuration`.

L'API bas niveau est `security.NewJWTService`.

```go
svc, err := security.NewJWTService(secret, 24*time.Hour)
if err != nil {
	return err
}

token, err := svc.Generate(map[string]any{
	"sub": "user-1",
	"roles": []string{"ADMIN"},
})
if err != nil {
	return err
}

claims, err := svc.Validate(token)
if err != nil {
	return err
}
_ = claims

fresh, err := svc.Refresh(token)
_ = fresh
```

Une duree d'expiration inferieure ou egale a zero utilise le defaut de 24h. Un secret vide retourne une erreur. Les erreurs utiles a tester avec `errors.Is` sont `security.ErrTokenExpired` et `security.ErrTokenInvalid`.

`Generate` signe en HS256 et ajoute le claim `exp`. `Validate` verifie la signature, l'algorithme et l'expiration. `Refresh` valide le token existant, retire l'ancien `exp` et produit un nouveau token avec une expiration remise a jour.

## Guards et RBAC

`security.NewJWTGuard` transforme un service JWT en `web.Guard`. Il attend strictement un header `Authorization: Bearer <token>`.

```go
guard := security.NewJWTGuard(svc)
if err := web.RegisterGuard(server, "authenticated", guard); err != nil {
	return err
}
```

Quand le token est valide, le guard stocke les claims dans `ctx.Locals("jwt_claims")`. Recuperer ces claims avec `security.ClaimsFromContext`.

```go
func CurrentUser(ctx web.Context) (string, bool) {
	claims, ok := security.ClaimsFromContext(ctx)
	if !ok {
		return "", false
	}
	sub, ok := claims["sub"].(string)
	return sub, ok
}
```

Le RBAC lit le claim `roles`. Le claim peut etre un `[]string` construit par votre code ou un `[]any` issu du parsing JSON/JWT.

Les APIs RBAC principales sont `security.NewRoleGuard` et `security.NewRoleGuardFactory`.

```go
if err := web.RegisterGuardFactory(server, "role", security.NewRoleGuardFactory()); err != nil {
	return err
}
```

Vous pouvez ensuite proteger une route avec une directive :

```go
//helix:guard authenticated
//helix:guard role:admin
func (c *AdminController) Index() ([]User, error) {
	return c.Service.Admins()
}
```

La forme exacte `//helix:guard role:admin` utilise la factory `role` avec l'argument `admin`.

`//helix:guard role:admin,moderator` accepte plusieurs roles separes par virgule. `security.NewRoleGuard("ADMIN")` est disponible pour un cablage manuel. N'appelez pas `security.NewRoleGuard()` sans role : cette API panique pour signaler une configuration invalide. Le builder global `security.HTTPSecurity.HasRole()` gere au contraire le cas zero role sans panique et refuse la requete.

## Security configurer

Pour definir une politique globale, creez un composant qui embed `helix.SecurityConfigurer` et implemente `security.Configurer`.

```go
type SecurityConfig struct {
	helix.SecurityConfigurer
}

func (SecurityConfig) Configure(hs *security.HTTPSecurity) {
	hs.Route("/actuator/**").PermitAll().
		Route("/api/**").Authenticated().
		Route("/admin/**").HasRole("ADMIN")
}
```

`helix.Run` detecte ce composant, construit `security.NewHTTPSecurity(jwtSvc)`, appelle `Configure`, puis applique le guard global avec `web.ApplyGlobalGuard`.

`security.NewHTTPSecurity` est aussi utilisable manuellement si vous assemblez les composants vous-meme :

Les methodes du builder a retenir sont `PermitAll`, `Authenticated` et `HasRole`.

```go
httpSecurity := security.NewHTTPSecurity(jwtSvc)
httpSecurity.Route("/actuator/**").PermitAll()
httpSecurity.Route("/api/**").Authenticated()

if err := web.ApplyGlobalGuard(server, httpSecurity.Build()); err != nil {
	return err
}
```

Les patterns supportent :

- `*` pour exactement un segment.
- `**` pour zero ou plusieurs segments.
- une correspondance exacte sans wildcard.

Les regles sont evaluees dans leur ordre de definition. La premiere regle correspondante gagne. Si aucune regle ne correspond, la requete continue. Le guard global s'execute avant les guards de route declares par `//helix:guard`, ce qui permet a la politique globale de court-circuiter une requete avant les guards locaux.

## Actuator

Les endpoints actuator sont enregistres via `observability.RegisterActuatorRoutes(server, checker, info)`.

`observability.NewCompositeHealthChecker` construit un checker a partir d'une liste d'indicateurs ; il retourne une erreur si la liste est vide ou si un indicateur est invalide.

```go
checker, err := observability.NewCompositeHealthChecker(&DatabaseHealth{})
if err != nil {
	return err
}

info := observability.NewInfoProvider(loader,
	observability.WithVersion("1.2.3"),
	observability.WithBuildInfo(map[string]string{"commit": "local"}),
)

if err := observability.RegisterActuatorRoutes(server, checker, info); err != nil {
	return err
}
```

`/actuator/health` retourne `200 OK` quand le statut global est `observability.StatusUp`, et `503 Service Unavailable` quand le statut global est `observability.StatusDown`.

```json
{"status":"UP"}
```

Un composant degrade apparait dans `components`.

```json
{
  "status": "DOWN",
  "components": {
    "database": {
      "status": "DOWN",
      "error": "connection refused"
    }
  }
}
```

Un indicateur de sante implemente `observability.HealthIndicator`.

Les APIs actuator principales sont `observability.HealthIndicator`, `observability.HealthCheckerFromContainer`, `observability.NewInfoProvider` et `observability.RegisterActuatorRoutes`.

```go
type DatabaseHealth struct{}

func (DatabaseHealth) Name() string { return "database" }

func (DatabaseHealth) Health(context.Context) observability.ComponentHealth {
	return observability.ComponentHealth{Status: observability.StatusUp}
}
```

`observability.HealthCheckerFromContainer(container)` construit un checker depuis les indicateurs enregistres dans le container. Si aucun indicateur n'existe, le health reste `UP` et la reponse n'inclut pas `components`.

`/actuator/info` est fourni par `observability.NewInfoProvider`. Les defaults sont stables : `version:"dev"`, `profiles:[]`, `build:{}`. `observability.WithVersion` et `observability.WithBuildInfo` permettent d'ajouter des informations de build explicites sans inventer de commit ou de date.

## Metriques Prometheus

`observability.Registry()` retourne le registre Prometheus Helix global. `observability.NewRegistry()` cree un registre isole, utile pour les tests.

Les APIs metriques principales sont `observability.Registry`, `observability.NewHTTPMetricsObserver`, `observability.RegisterMetricsRoute` et `observability.WithMetricsGuard`.

Pour observer les routes HTTP, branchez `observability.NewHTTPMetricsObserver`.

```go
registry := observability.Registry()
observer, err := observability.NewHTTPMetricsObserver(registry)
if err != nil {
	return err
}
server.AddRouteObserver(observer)
```

Les metriques principales sont :

- `helix_http_requests_total`
- `helix_http_request_duration_seconds`

Exposez le format texte Prometheus avec `observability.RegisterMetricsRoute`.

```go
if err := observability.RegisterMetricsRoute(server, observability.Registry()); err != nil {
	return err
}
```

`/actuator/metrics` est public par defaut. Pour le proteger, passez `observability.WithMetricsGuard`.

```go
if err := observability.RegisterMetricsRoute(
	server,
	observability.Registry(),
	observability.WithMetricsGuard(security.NewJWTGuard(jwtSvc)),
); err != nil {
	return err
}
```

## Logging structure

`observability.ConfigureLogging(loader)` configure un logger JSON et l'installe avec `slog.SetDefault`. Cette fonction modifie l'etat global de `slog` ; elle n'est pas thread-safe et doit etre appelee une seule fois au demarrage de l'application.

Les APIs logging principales sont `observability.ConfigureLogging` et `observability.Logger`.

Les cles de configuration sont `helix.logging.level` et `helix.logging.levels`.

```yaml
helix:
  logging:
    level: "info"
    levels:
      web: "debug"
      data: "warn"
```

Les niveaux supportes sont `debug`, `info`, `warn`, `error`. Le JSON contient un champ `timestamp` et un champ `namespace`.

```go
logger, err := observability.ConfigureLogging(loader)
if err != nil {
	return err
}

logger.Info("application started")
observability.Logger("web").Debug("route registered", "path", "/users")
observability.Logger("data").With("repository", "users").Warn("slow query")
```

Pour les tests, `observability.WithLoggingOutput`, `observability.WithDefaultNamespace` et `observability.WithLoggingConfig` permettent d'eviter stdout et de fournir une configuration explicite.

## Tracing OpenTelemetry

`observability.ConfigureTracing(loader)` initialise OpenTelemetry quand le tracing est active. Par defaut, il est desactive et la fonction retourne `(nil, nil, nil)`.

L'API tracing principale est `observability.ConfigureTracing`. Quand une fonction est retournee, appelez `shutdown(ctx)` pendant l'arret.

```yaml
helix:
  starters:
    observability:
      tracing:
        enabled: true
        exporter: "stdout"
        endpoint: "localhost:4318"
        service-name: "users-api"
```

Les exporters supportes sont `stdout`, `otlp` et `jaeger`. L'exporter `jaeger` utilise le chemin OTLP HTTP vers l'endpoint configure.

```go
provider, shutdown, err := observability.ConfigureTracing(loader)
if err != nil {
	return err
}
if shutdown != nil {
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	}()
}
_ = provider
```

Les cles lues sont `helix.starters.observability.tracing.enabled`, `helix.starters.observability.tracing.exporter`, `helix.starters.observability.tracing.endpoint` et `helix.starters.observability.tracing.service-name`. `observability.WithTracingConfig` et `observability.WithTracingOutput` servent aux tests et au cablage manuel.

## Scheduling

`scheduler.NewScheduler()` retourne un scheduler base sur `robfig/cron/v3`. Il implemente `core.Lifecycle` via `OnStart` et `OnStop`, ce qui permet au container de demarrer et arreter le runner.

Les APIs scheduling principales sont `scheduler.NewScheduler`, `scheduler.Job`, `scheduler.ScheduledJobProvider`, `scheduler.WrapError` et `scheduler.WrapSkipIfBusy`. La configuration `helix.starters.scheduling.enabled: true` permet de forcer l'activation du starter.

```go
sched := scheduler.NewScheduler()

err := sched.Register(scheduler.Job{
	Name: "cleanup", # Le nom doit etre unique pour eviter les collisions
	Expr: "@every 1h",
	Fn: func() {
		// cleanup
	},
})
if err != nil {
	return err
}
```

`scheduler.Job` contient `Name`, `Expr`, `Fn` et `AllowConcurrent`. `scheduler.CronExpression` accepte les expressions cron 5 champs et les shortcuts robfig comme `@every 1h` ou `@hourly`. Un `Fn` nil est refuse avec `scheduler.ErrInvalidCron`.

Pour logger les erreurs d'une fonction qui retourne `error`, utilisez `scheduler.WrapError`.

```go
job := scheduler.Job{
	Name: "sync-users",
	Expr: "@hourly",
	Fn: scheduler.WrapError("sync-users", func() error {
		return nil
	}),
}
```

`scheduler.WrapSkipIfBusy` ignore une execution si la precedente tourne encore. Le starter scheduling applique ce wrapper par defaut quand `AllowConcurrent` est faux.

Le chemin runtime actuel pour declarer des jobs depuis un composant est `scheduler.ScheduledJobProvider`.

```go
type CleanupJobs struct{}

func (CleanupJobs) ScheduledJobs() []scheduler.Job {
	return []scheduler.Job{{
		Name: "cleanup",
		Expr: "@every 1h",
		Fn: scheduler.WrapError("cleanup", func() error {
			return nil
		}),
	}}
}
```

La directive `//helix:scheduled` est une convention scannee par le codegen. **Elle n'a aucun effet au runtime** sans une execution prealable de `helix generate`. Aujourd'hui, l'enregistrement runtime explicite passe par `ScheduledJobProvider`; gardez la directive proche de la methode source si vous voulez aligner votre code avec la generation.

```go
//helix:scheduled @every 1h
func (CleanupJobs) Cleanup() error {
	return nil
}
```

## Starters

Le starter security enregistre `*security.JWTService` quand la configuration contient `security.jwt.secret`. Le starter observability configure logging, tracing, `/actuator/health`, `/actuator/info` et `/actuator/metrics` quand il trouve une configuration `observability` ou `helix.starters.observability.enabled`. Le starter scheduling enregistre `scheduler.NewScheduler()` et les providers si `robfig/cron` est present dans `go.mod`.

```yaml
helix:
  starters:
    observability:
      enabled: true
    scheduling:
      enabled: true
```

`helix.starters.scheduling.enabled` controle le starter scheduling, mais la presence de `robfig/cron` dans `go.mod` reste necessaire pour l'activation automatique.

## Tests

Les tests unitaires des guards peuvent utiliser `web.NewServer`, `web.RegisterGuard` et `web.RegisterGuardFactory`. Les tests actuator peuvent enregistrer les routes sur un serveur Helix in-memory et appeler `ServeHTTP`.

```bash
go test ./security/...
go test ./observability/...
go test ./scheduler/...
go test ./starter/security ./starter/observability ./starter/scheduling
```

Pour les metriques, preferez `observability.NewRegistry()` afin d'eviter les collisions entre tests. Pour les logs, utilisez `observability.WithLoggingOutput`. Pour le tracing, gardez `stdout` et un writer de test via `observability.WithTracingOutput`.

## Erreurs frequentes

**Secret JWT vide** : `security.NewJWTService("", 24*time.Hour)` retourne une erreur et le starter ne peut pas fournir de service JWT utilisable.

**Header Authorization incomplet** : `security.NewJWTGuard` attend `Authorization: Bearer <token>`, avec un token non vide.

**Claim roles mal forme** : `security.RoleGuard` attend `roles` sous forme de liste de strings.

**Ordre des regles globales** : dans `security.HTTPSecurity`, la premiere regle qui correspond gagne. Placez les exceptions publiques avant les patterns larges.

**Metrics exposees publiquement** : `observability.RegisterMetricsRoute` est public par defaut ; utilisez `observability.WithMetricsGuard` si votre environnement l'exige.

**Tracing active sans shutdown** : si `observability.ConfigureTracing` retourne une fonction de shutdown, appelez-la pendant l'arret de l'application.

**Confondre directive et runtime scheduling** : `//helix:scheduled` documente l'intention et la convention codegen ; le chemin runtime explicite actuel reste `scheduler.ScheduledJobProvider`.
