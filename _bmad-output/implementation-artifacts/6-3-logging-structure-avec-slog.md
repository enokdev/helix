# Story 6.3: Logging Structure avec `slog`

Status: done

<!-- Note: Validation optionnelle. Cette story a ete creee par le workflow create-story et relue contre la checklist locale. -->

## Story

En tant que **developpeur utilisant Helix**,
Je veux un logging structure JSON configure automatiquement,
Afin de pouvoir interroger mes logs en production avec des outils comme Loki ou CloudWatch.

## Acceptance Criteria

1. **Given** la configuration Helix est disponible, **When** le logging Helix est configure, **Then** `slog` utilise un handler JSON standard library; **And** aucun logger tiers (`zap`, `zerolog`, `logrus`, etc.) n'est ajoute.
2. **Given** aucune configuration explicite n'est fournie, **When** le logger Helix est installe, **Then** le niveau global par defaut est `info`, le format est JSON et la sortie par defaut est `os.Stdout`.
3. **Given** `helix.logging.level: debug|info|warn|error`, **When** le logger est configure depuis `config.Loader`, **Then** le niveau global filtre correctement les logs sous ce seuil; **And** une valeur invalide retourne une erreur wrappee sans `panic()`.
4. **Given** `helix.logging.levels.web: debug`, **When** un logger namespace `web` emet un log `DEBUG`, **Then** ce log est emis meme si le niveau global est plus restrictif; **And** un autre namespace conserve le niveau global.
5. **Given** un log est emis via `slog.Default()` ou via un logger namespace Helix, **When** la sortie JSON est decodee, **Then** chaque entree contient exactement les champs top-level `timestamp`, `level`, `msg` et `namespace`; **And** `namespace` vaut `app` pour le logger par defaut si aucun namespace explicite n'est fourni.
6. **Given** `slog.Default()` est appele apres configuration, **When** le developpeur utilise `slog.Info`, `slog.Warn` ou `slog.Default().With(...)`, **Then** ces appels utilisent le logger Helix configure et produisent du JSON compatible avec les AC precedents.
7. **Given** les composants existants `core` et `config` utilisent deja `slog.Default()` ou un `*slog.Logger` optionnel, **When** le logger Helix est configure avant leur utilisation, **Then** leurs logs suivent le format JSON Helix sans introduire de dependance Helix supplementaire dans `core/` ou `config/`.
8. **Given** une erreur de serialisation JSON survient dans la reponse HTTP de succes, **When** `web.writeSuccessResponse` retourne l'erreur, **Then** l'erreur est journalisee via `slog` avec namespace `web`, sans exposer de body sensible et sans changer le status mapping existant.
9. **Given** la suite de tests, **When** le logging est verifie, **Then** les tests couvrent le niveau global, le niveau par namespace, les valeurs invalides, `slog.Default()`, le champ `timestamp`, le namespace par defaut, l'absence de duplicate `namespace`, et le logging de l'erreur JSON dans `web`.

## Tasks / Subtasks

- [x] Tache 1: Definir le contrat public de logging dans `observability/logging.go` (AC: #1, #2, #3, #5, #6)
  - [x] Ajouter `observability/logging.go` dans le package `observability`.
  - [x] Definir un type public minimal, par exemple `type LoggingConfig struct { Level string; Levels map[string]string }`.
  - [x] Definir une API de configuration explicite, par exemple `ConfigureLogging(loader config.Loader, opts ...LoggingOption) (*slog.Logger, error)`.
  - [x] `ConfigureLogging` doit appeler `slog.SetDefault(logger)` uniquement apres validation complete de la config.
  - [x] Ajouter des options testables: `WithLoggingOutput(io.Writer)`, `WithDefaultNamespace(string)`, et si necessaire `WithLoggingConfig(LoggingConfig)` pour les tests sans fichier YAML.
  - [x] Refuser les options nil, writer nil, namespace vide et niveaux invalides avec `ErrInvalidLogging`, wrappe par `fmt.Errorf("observability: configure logging: %w", err)`.

- [x] Tache 2: Parser la configuration `helix.logging.*` depuis `config.Loader` (AC: #2, #3, #4)
  - [x] Lire `helix.logging.level` via `loader.Lookup("helix.logging.level")`; si absent, utiliser `info`.
  - [x] Lire `helix.logging.levels` depuis `loader.Lookup("helix.logging.levels")` ou `loader.AllSettings()` pour supporter un YAML map comme `levels: { web: debug }`.
  - [x] Ajouter les cles connues minimales dans `config` pour les ENV-only keys: `helix.logging.level` et `helix.logging.levels.web`.
  - [x] Ne pas creer de dependance de `config/` vers `observability/`; `config` reste un package de fondation autonome.
  - [x] Accepter uniquement `debug`, `info`, `warn`, `error` en entree utilisateur, en comparaison case-insensitive et trimmee.
  - [x] Mapper les niveaux vers `slog.LevelDebug`, `slog.LevelInfo`, `slog.LevelWarn`, `slog.LevelError`.

- [x] Tache 3: Implementer le handler JSON avec niveau global et niveaux par namespace (AC: #1, #3, #4, #5)
  - [x] Utiliser `slog.NewJSONHandler(output, &slog.HandlerOptions{Level: ...})` comme handler de base.
  - [x] Utiliser `slog.LevelVar` ou un `slog.Leveler` equivalent pour le niveau global.
  - [x] Ajouter un wrapper de handler interne, par exemple `namespaceLevelHandler`, pour appliquer les seuils par namespace.
  - [x] Le `Enabled(ctx, level)` du wrapper doit autoriser le niveau minimum parmi global et namespaces configures, afin qu'un namespace `debug` ne soit pas coupe avant `Handle`.
  - [x] Le `Handle(ctx, record)` du wrapper doit determiner le namespace effectif, appliquer le seuil final, puis deleguer au handler JSON.
  - [x] Le wrapper doit implementer correctement `WithAttrs` et `WithGroup`; attention aux attributs portes par `logger.With(...)`, car ils ne sont pas visibles via les seuls attrs du `Record`.
  - [x] Eviter les duplicate keys JSON pour `namespace`; si un logger ajoute deja `namespace`, la valeur explicite doit remplacer le namespace par defaut au lieu de produire deux champs identiques.

- [x] Tache 4: Adapter les attributs JSON pour le contrat Helix (AC: #5, #6)
  - [x] `slog.JSONHandler` emet `time` par defaut; utiliser `HandlerOptions.ReplaceAttr` pour renommer la cle root `slog.TimeKey` en `timestamp`.
  - [x] Conserver les cles standard `level` et `msg`.
  - [x] Injecter `namespace:"app"` quand aucun namespace explicite n'est fourni.
  - [x] Fournir une API de logger namespace, par exemple `observability.Logger(namespace string) *slog.Logger` ou `observability.WithNamespace(logger, namespace) *slog.Logger`.
  - [x] Ne pas utiliser `AddSource` pour satisfaire le namespace; le namespace attendu est une valeur logique (`web`, `data`, `app`), pas un chemin fichier/ligne.

- [x] Tache 5: Integrer proprement avec les packages existants (AC: #6, #7, #8)
  - [x] Ne pas modifier `core/` pour importer `observability`; `core` doit continuer a accepter `core.WithLogger(*slog.Logger)` et a tomber sur `slog.Default()` par defaut.
  - [x] Ne pas modifier `config/` pour importer `observability`; `config.Reloader` doit continuer a utiliser `slog.Default()` sauf si `WithReloadLogger` est fourni.
  - [x] Ne pas brancher le starter observability ici; l'auto-configuration des starters reste Epic 7.
  - [x] Ajouter dans `web/response.go` un log d'erreur pour `writeSuccessResponse` quand `ctx.JSON(payload)` retourne une erreur; utiliser un logger namespace `web`.
  - [x] Ne pas logger le payload complet en cas d'erreur JSON; journaliser au plus le type Go du payload, la methode HTTP si disponible, et l'erreur.
  - [x] Ne pas changer le contrat public de `web.Context` sauf necessite forte; la story peut journaliser sans connaitre le body complet.

- [x] Tache 6: Ajouter les tests co-localises pour `observability` (AC: #1-#6, #9)
  - [x] Ajouter `observability/logging_test.go`.
  - [x] Utiliser `bytes.Buffer` via `WithLoggingOutput(&buf)`; ne pas ecrire sur stdout/stderr dans les tests.
  - [x] Restaurer `slog.Default()` apres chaque test qui appelle `ConfigureLogging`.
  - [x] Tester que le JSON decode contient `timestamp`, `level`, `msg`, `namespace`.
  - [x] Tester `helix.logging.level: warn` filtre `debug` et `info`, mais laisse `warn` et `error`.
  - [x] Tester `helix.logging.levels.web: debug` laisse passer un debug `web` sans laisser passer un debug `data`.
  - [x] Tester les valeurs invalides (`trace`, `fatal`, `verbose`, chaine vide explicite) et verifier `errors.Is(err, ErrInvalidLogging)`.
  - [x] Tester qu'un logger namespace ne produit pas deux cles `namespace` dans le JSON brut.

- [x] Tache 7: Ajouter les tests co-localises pour l'integration `config` et `web` (AC: #3, #4, #8, #9)
  - [x] Ajouter ou completer des tests `config` pour prouver que `HELIX_LOGGING_LEVEL` et `HELIX_LOGGING_LEVELS_WEB` sont visibles quand aucune cle YAML n'existe.
  - [x] Ajouter un test `observability` ou `config` qui charge un YAML imbrique `helix.logging.levels.web: debug`.
  - [x] Ajouter un test `web` qui provoque une erreur JSON sur un payload non serialisable, verifie que l'erreur remonte toujours et qu'une ligne JSON namespace `web` est emise.
  - [x] Garder les tests table-driven quand plusieurs niveaux ou cas invalides sont verifies.

- [x] Tache 8: Verification locale finale
  - [x] Executer `gofumpt -w observability web config` si disponible; sinon `gofmt -w` sur les fichiers touches.
  - [x] Executer `go test ./observability/...`.
  - [x] Executer `go test ./config/...`.
  - [x] Executer `go test ./web/...`.
  - [x] Executer `go test ./...`.
  - [x] Executer `go build ./...`.
  - [x] Executer `go vet ./...`.
  - [x] Executer `golangci-lint run` si disponible localement; sinon noter explicitement son absence.
  - [x] Verifier qu'aucun terme de suivi interne n'a ete ajoute aux fichiers Go, README ou CONTRIBUTING.

## Dev Notes

### Perimetre strict de cette story

Inclus:

- configuration JSON `slog` via une API publique dans `observability`;
- niveau global `helix.logging.level`;
- niveaux par namespace `helix.logging.levels.<namespace>`;
- namespace logique dans chaque entree JSON;
- `slog.Default()` configure par Helix;
- tests de filtrage global et par namespace;
- correction du deferred work sur l'erreur JSON success non loguee dans `web.writeSuccessResponse`.

Hors perimetre:

- auto-activation par starter `observability`: Epic 7;
- tracing OpenTelemetry et trace IDs dans le contexte: story 6.4;
- integration Loki/CloudWatch reelle;
- rotation de fichiers, multi-sinks, sampling, redacteurs de secrets generalises;
- remplacement de `slog` par une librairie tierce;
- ajout d'un middleware Fiber ou d'un import Fiber hors `web/internal`;
- changement de `helix.Run` pour charger toute la configuration YAML automatiquement.

### API cible recommandee

La story doit fournir une API composable que le futur starter pourra appeler:

```go
loader := config.NewLoader(config.WithAllowMissingConfig())
if err := loader.Load(new(struct{})); err != nil {
	return err
}

logger, err := observability.ConfigureLogging(loader)
if err != nil {
	return err
}

container := core.NewContainer(
	core.WithResolver(core.NewReflectResolver()),
	core.WithLogger(logger),
)
```

Pour un test ou une configuration explicite sans YAML:

```go
var logs bytes.Buffer

logger, err := observability.ConfigureLogging(
	nil,
	observability.WithLoggingOutput(&logs),
	observability.WithLoggingConfig(observability.LoggingConfig{
		Level: "warn",
		Levels: map[string]string{
			"web": "debug",
		},
	}),
)
if err != nil {
	return err
}

observability.Logger("web").Debug("route matched", "route", "/users/:id")
logger.Warn("application warning")
```

Adapter les noms exacts au style final, mais conserver les principes: `observability` configure `slog`, `config` fournit les valeurs, les packages fondation restent decouples.

### Contrat JSON attendu

Exemple par defaut:

```json
{"timestamp":"2026-04-19T12:00:00Z","level":"INFO","msg":"application started","namespace":"app"}
```

Exemple namespace:

```json
{"timestamp":"2026-04-19T12:00:01Z","level":"DEBUG","msg":"route matched","namespace":"web","route":"/users/:id"}
```

Notes importantes:

- `slog.JSONHandler` utilise `time` par defaut; l'AC demande `timestamp`, donc `ReplaceAttr` est obligatoire.
- Le champ `namespace` doit etre top-level pour faciliter les requetes Loki/CloudWatch.
- Ne pas logger de secrets, tokens, DSN, headers complets ou bodies complets dans les logs d'erreur framework.
- Les logs `slog.Info(...)` top-level doivent aussi produire `namespace:"app"`.

### Etat actuel du code a reutiliser

- `observability/doc.go` mentionne deja Prometheus, slog et OTel; `observability/logging.go` est le bon emplacement prevu par l'architecture.
- `core.Container` initialise deja son logger avec `slog.Default()` et expose `core.WithLogger`.
- `config.Reloader` initialise deja son logger avec `slog.Default()` et expose `WithReloadLogger`.
- `helix.App` expose deja `Logger *slog.Logger`, transmis a `core.WithLogger`.
- `config.Loader` expose `Lookup`, `AllSettings` et `ActiveProfiles`.
- `config/reload.go` contient `knownConfigKeys`, aujourd'hui limite a `helix.config.reload-interval`; ce mecanisme est le bon endroit pour rendre les cles logging ENV-only visibles.
- `web.writeSuccessResponse` retourne actuellement directement `ctx.JSON(payload)`; le deferred work `D-3.5-4` demande d'ajouter un log quand cette serialisation echoue.
- `web/` peut importer `log/slog` (stdlib), mais ne doit pas importer `observability`.

### Decision critique: handler namespace plutot que plusieurs loggers globaux

Le filtrage par namespace ne peut pas etre resolu seulement avec `slog.HandlerOptions.Level`, car `Enabled(ctx, level)` ne recoit pas les attributs du record. Une implementation robuste doit:

- laisser passer dans `Enabled` le niveau minimum configure globalement ou par namespace;
- lire le namespace dans `Handle`, en tenant compte des attrs persistants ajoutes par `Logger.With`;
- appliquer le seuil final dans `Handle`;
- injecter exactement un champ `namespace` avant de deleguer au JSON handler.

Cette approche evite:

- de dupliquer plusieurs handlers globaux;
- de perdre les logs `DEBUG` d'un namespace specifique;
- de produire du JSON invalide ou ambigu avec plusieurs cles `namespace`.

### Decision critique: ne pas implementer le starter maintenant

L'architecture prevoit `starter/observability` plus tard. Cette story doit fournir les briques logging que ce starter appellera, sans introduire maintenant l'auto-configuration complete:

- pas de nouvelle dependance entre `helix.Run` et `observability`;
- pas de lecture magique de YAML au bootstrap root;
- pas de changement de comportement pour les applications qui n'appellent pas encore la configuration logging.

Cette limite garde la story compatible avec les stories 6.1 et 6.2, qui ont elles aussi fourni des briques composables pour le futur starter.

### Intelligence issue de la story precedente et du git recent

- La story 6.2 a ajoute Prometheus v1.21.1 pour rester compatible avec `go 1.21.0`; cette story ne doit ajouter aucune dependance externe.
- La story 6.2 a etabli le pattern `observability` orchestre, `web` expose une abstraction generique, et `web/internal` reste le seul package Fiber.
- La review 6.2 a insiste sur les guards nil/typed-nil et le rollback propre quand une etape partielle echoue; appliquer la meme discipline aux options logging.
- Les stories 6.1 et 6.2 ont evite de brancher les starters; conserver cette frontiere pour ne pas anticiper Epic 7.
- Les commits recents montrent que les corrections de review preferent des erreurs wrappees, des tests d'erreurs explicites et aucun `panic()`.
- `deferred-work.md` contient `D-3.5-4`: l'echec de serialisation JSON success remonte en 500 mais n'est pas logue. Cette story est le bon moment pour instrumenter ce point avec namespace `web`.

### Latest Technical Information

- Recherche effectuee le 2026-04-19: `log/slog` est dans la bibliotheque standard depuis Go 1.21; c'est le choix exact de l'architecture Helix, donc ne pas ajouter de logger tiers. Source: https://go.dev/blog/slog
- Documentation officielle consultee le 2026-04-19: `slog.NewJSONHandler(os.Stdout, nil)` produit des objets JSON avec les cles standard `time`, `level` et `msg`; Helix doit renommer `time` en `timestamp` pour respecter l'AC. Source: https://pkg.go.dev/log/slog
- Documentation officielle consultee le 2026-04-19: `slog.SetDefault(logger)` fait utiliser ce logger par les fonctions top-level `slog.Info`, `slog.Debug`, etc., et connecte aussi le logger standard `log` au handler configure. Source: https://pkg.go.dev/log/slog#SetDefault
- Documentation officielle consultee le 2026-04-19: `slog.LevelVar` est safe pour usage concurrent et permet de changer dynamiquement le niveau d'un handler; il convient au niveau global. Source: https://pkg.go.dev/log/slog#LevelVar

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-6.3-Logging-Structure-avec-slog]
- [Source: _bmad-output/planning-artifacts/epics.md#FR19]
- [Source: _bmad-output/planning-artifacts/architecture.md#Contraintes-Techniques-Dependances]
- [Source: _bmad-output/planning-artifacts/architecture.md#Structure-du-Depot-Module-Unique-Phase-1-2]
- [Source: _bmad-output/planning-artifacts/architecture.md#Mapping-Exigences-PRD-Structure]
- [Source: _bmad-output/implementation-artifacts/6-1-endpoints-actuator-health-info.md]
- [Source: _bmad-output/implementation-artifacts/6-2-metriques-prometheus-actuator-metrics.md]
- [Source: _bmad-output/implementation-artifacts/deferred-work.md#D-3.5-4]
- [Source: go.mod]
- [Source: observability/doc.go]
- [Source: core/container.go]
- [Source: core/options.go]
- [Source: config/loader.go]
- [Source: config/reload.go]
- [Source: web/response.go]
- [Source externe: Go slog blog, consultee le 2026-04-19: https://go.dev/blog/slog]
- [Source externe: log/slog package docs, consultee le 2026-04-19: https://pkg.go.dev/log/slog]

## Dev Agent Record

### Agent Model Used

claude-sonnet-4-6

### Debug Log References

- Race condition sur `slog.Default()` : tests parallèles qui appellent `ConfigureLogging` (qui appelle `slog.SetDefault`) doivent éviter `t.Parallel()`. Les tests de validation d'erreurs (sans `slog.SetDefault`) restent parallèles.
- `parseLevel("")` : la chaîne vide après trim est invalide (ErrInvalidLogging) ; le défaut `"info"` est injecté par `resolveLoggingConfig`, pas par `parseLevel`.
- `golangci-lint` absent localement. `go vet ./...` passe sans erreur.

### Completion Notes List

- `observability/logging.go` : API publique complète — `ErrInvalidLogging`, `LoggingConfig`, `LoggingOption`, `ConfigureLogging`, `Logger`, `WithLoggingOutput`, `WithDefaultNamespace`, `WithLoggingConfig`. Handler interne `namespaceLevelHandler` implémente `slog.Handler` avec filtrage par namespace, renommage `time→timestamp`, injection d'un seul champ `namespace`.
- `config/reload.go` : `knownConfigKeys` étendu avec `helix.logging.level` et `helix.logging.levels.web` pour que les vars ENV soient visibles sans YAML.
- `web/response.go` : `writeSuccessResponse` log l'erreur JSON avec namespace `web` via `slog.Default()` (type Go du payload, méthode HTTP, erreur). Pas de changement du contrat public de `web.Context`.
- Tous les ACs satisfaits. Suite complète `go test ./...` verte. `go build ./...` et `go vet ./...` sans erreur.

### File List

- observability/logging.go (nouveau)
- observability/logging_test.go (nouveau)
- web/response.go (modifié)
- web/response_test.go (nouveau)
- config/reload.go (modifié)
- config/loader_test.go (modifié)
- _bmad-output/implementation-artifacts/sprint-status.yaml (modifié)

### Review Findings

- [x] [Review][Decision] AC5 — Champs supplémentaires dans le log d'erreur web — `web/response.go:27-31` ajoute `payload_type`, `method`, `error` au log; AC5 exige « exactement » `timestamp`, `level`, `msg`, `namespace`. Décider si les champs de diagnostic supplémentaires sont autorisés dans les logs d'erreur framework. → **Résolu : autorisés pour les logs d'erreur internes framework.**
- [x] [Review][Patch] Double-émission des attrs `With(...)` dans le JSON [observability/logging.go:203-208,191-195] — `WithAttrs` passe les attrs à `clone.preAttrs` ET à `clone.inner.WithAttrs`; `Handle` ré-ajoute ensuite `preAttrs` à `nr` avant de déléguer à `inner.Handle`. Chaque attr non-namespace de `logger.With(...)` apparaît deux fois dans le JSON final.
- [x] [Review][Patch] `resolveNamespace` retourne le premier preAttr correspondant au lieu du dernier [observability/logging.go:235-238] — `logger.With("namespace","A").With("namespace","B")` résout `"A"` au lieu de `"B"`. Corriger pour itérer en sens inverse.
- [x] [Review][Patch] `WithLoggingConfig` alias le map `Levels` du caller sans deep copy [observability/logging.go:58-63] — `cfg` est pris par valeur mais `cfg.Levels` est un `map[string]string` (type référence). Mutations post-appel du caller visibles dans la config stockée. Ajouter une copie du map.
- [x] [Review][Patch] `ConfigureLogging` godoc ne documente pas l'effet de bord `slog.SetDefault` [observability/logging.go:67] — Ajouter une phrase au commentaire pour avertir que `slog.SetDefault` est appelé.
- [x] [Review][Patch] Test invalide manquant : chaîne vide `""` dans la table-driven des niveaux invalides [observability/logging_test.go:267] — La table couvre `"  "` (whitespace) mais pas `""` littérale. Ajouter `""` comme cas de test.
- [x] [Review][Defer] `WithGroup` imbrique le champ `namespace` dans le groupe JSON [observability/logging.go:210-213] — `WithGroup` ne trace pas l'état du groupe dans le handler; l'injection de `namespace` dans `Handle` tombe à l'intérieur du groupe. Correction non-triviale; usage rare dans les patterns Helix. — deferred, pre-existing
- [x] [Review][Defer] `knownConfigKeys` couvre seulement `helix.logging.levels.web` [config/reload.go:19] — Les ENV vars pour d'autres namespaces (ex. `HELIX_LOGGING_LEVELS_DATA`) ne sont jamais bindées. La spec demande le minimal (web seulement); extension à d'autres namespaces est hors périmètre story. — deferred, spec-minimal
- [x] [Review][Defer] Valeurs non-string dans namespace levels silencieusement ignorées [observability/logging.go:289-292] — Si YAML contient `web: 1` (entier), l'assertion `.(string)` échoue silencieusement. Corriger nécessite un changement de signature de `resolveLoggingConfig`. — deferred, pre-existing
- [x] [Review][Defer] `writeSuccessResponse` fixe le HTTP status avant que `ctx.JSON` réussisse [web/response.go:25-26] — Pattern pré-existant; le status peut être envoyé avant échec de sérialisation. Non introduit par cette story. — deferred, pre-existing

## Change Log

- 2026-04-19 : Implémentation de la Story 6.3 — Logging structuré avec `slog` : API `ConfigureLogging`, handler JSON avec niveau global et niveaux par namespace, renommage `time→timestamp`, `slog.Default()` configuré, log erreur JSON dans `web`, clés ENV logging dans `config`.

## Story Context Completion Status

Ultimate context engine analysis completed - comprehensive developer guide created.
