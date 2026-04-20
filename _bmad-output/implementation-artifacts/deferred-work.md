
## Deferred from: code review of 7-2-starter-web-auto-activation-fiber (2026-04-19)

- [D-7.2-1] `Condition()` lit `go.mod` depuis le CWD — en production sans go.mod adjacent, le starter ne s'active jamais ; décision documentée dans les Dev Notes avec justification explicite. À reconsidérer si un mécanisme de détection compile-time (build tag ou interface check) devient disponible. [starter/web/starter.go:35]
- [D-7.2-2] Aucune validation de plage du port (`"0"`, `"99999"`, chaîne non numérique) — `formatPort` retourne la valeur as-is pour les strings ; l'erreur remonte uniquement au moment du `server.Start()`. Ajouter une validation `strconv.Atoi` + range check dans `formatPort` si les UX-errors au démarrage deviennent prioritaires. [starter/web/starter.go:58-64]
- [D-7.2-3] Mutation globale du CWD dans les tests (`os.Chdir`) — risque de race si `t.Parallel()` est ajouté à l'avenir. Mitigation actuelle : aucun `t.Parallel()` dans ce package. À traiter si la suite de tests est parallélisée. [starter/web/starter_test.go]
- [D-7.2-4] Pas de test end-to-end `helix.Run()` avec `WebStarter` intégré — couvre AC 1 mais nécessite un serveur Fiber actif en test ; hors périmètre story 7.2 (intégration auto-discovery dans Run réservée story future). [helix.go]

## Deferred from: code review of 5-2-helix-mockbean-remplacement-automatique-par-mock (2026-04-19)

- [D-5.2-1] Composant multi-interfaces filtré entièrement par `MockBean[A]` → interface B non couverte devient unresolvable avec message sans contexte mock. Limitation architecturale du filtrage par assignabilité ; à adresser si des composants multi-interfaces deviennent courants dans les tests. [testutil/mock.go:71-82]
- [D-5.2-2] Mock impl satisfaisant des interfaces supplémentaires au-delà de la cible T → `ErrUnresolvable` "multiple registrations" au démarrage du container sans mention de la cause mock. Limitation du resolver existant ; à améliorer avec un message d'erreur enrichi dans core. [testutil/app.go:58-62]
- [D-5.2-3] Éventuel wrapper `ComponentRegistration` passé via `WithComponents` contourne `isReplacedComponent` car `reflect.TypeOf` voit le wrapper, pas le type interne. Non déclenché par l'API publique actuelle ; à traiter si une API ComponentRegistration publique est introduite. [testutil/mock.go:71-82]

## Deferred from: code review of 4-1-interface-repository-generique (2026-04-18)

- [D-4.1-1] Sanitization de `Condition.Field` — risque d'injection pour les adaptateurs qui interpolent le nom de colonne dans le SQL (colonnes non paramétrables) ; responsabilité de l'adaptateur GORM (Story 4.2). [data/filter.go:44-45]
- [D-4.1-2] `OperatorContains` — échappement des wildcards `%` et `_` non documenté au niveau contrat ; les adaptateurs doivent escaper avant de construire le `LIKE ?`. À documenter ou enforcer dans Story 4.2. [data/filter.go:22]
- [D-4.1-3] `FindAll()` sans borne — charge toute la table en mémoire ; choix API délibéré conforme au spec ; les consommateurs doivent utiliser `Paginate` pour les grands volumes. À documenter dans le guide dev. [data/repository.go:5]
- [D-4.1-4] `testRepository.FindWhere` ignore le filtre passé — stub compile-time uniquement ; tests comportementaux (filtre effectivement appliqué) délégués à Story 4.2 avec l'implémentation GORM réelle. [data/repository_test.go:28-30]
- [D-4.1-5] `Paginate` — valeurs négatives ou zéro pour `page`/`size` non rejetées au niveau de l'interface ; validation à enforcer dans chaque implémentation concrète (Story 4.2). [data/repository.go:10]
- [D-4.1-6] Type mismatch `Value` vs opérateur (ex: `string` passé à `OperatorGreaterThan`) — `any` est intentionnel pour la portabilité ORM-neutral ; l'adaptateur doit valider/coercer le type concret. [data/filter.go:46-47]
- [D-4.1-7] `Page.Total` négatif possible — aucun invariant au niveau contrat ; les implémentations doivent s'assurer que `Total >= 0` (Story 4.2). [data/pagination.go:6]
- [D-4.1-8] `Unwrap()` peut retourner nil — interface `Transaction` ne documente pas si nil est une valeur de retour valide ; les adaptateurs qui font une type assertion bare paniquent. À clarifier dans la doc de Story 4.2. [data/transaction.go:5]
- [D-4.1-9] `Page.Total` est `int` — conforme au spec ; `int` est 64 bits sur toutes les cibles modernes (linux/amd64, darwin/arm64) ; à reconsidérer uniquement si un support 32 bits est requis. [data/pagination.go:6]

## Deferred from: code review of 3-7-guards-interceptors-declaratifs (2026-04-17)

- [D-3.7-1] Changement cassant sur l'interface publique `Context` (`Method()`, `OriginalURL()`) — requis par spec, doubles de test mis à jour dans `web/internal/` ; les implémentations externes doivent être mises à jour. [web/context.go]
- [D-3.7-2] Guards coupent la chaîne d'interceptors pour les requêtes rejetées — comportement by design explicite dans la spec ; interceptors de type "observe-all" (tracing, métriques) ne verront pas les requêtes refusées. [web/router.go:composeHandler]
- [D-3.7-3] Cache stampede — N goroutines concurrentes sur cold cache key exécutent toutes le handler (pas de single-flight) ; à adresser si le cache devient un composant de premier plan. [web/cache_interceptor.go]
- [D-3.7-4] `responseRecorder` : `Status(non-2xx)` appelé après `JSON()` empêche la mise en cache d'une réponse 200 déjà envoyée — pattern inhabituel, à documenter. [web/cache_interceptor.go]
- [D-3.7-5] Croissance unbounded du cache sans limite de taille ni sweep proactif — les entrées expirées ne sont évincées que paresseusement sur le prochain `get` ; à traiter si le cache devient production-grade. [web/cache_interceptor.go]
- [D-3.7-6] Fichiers de test importent `github.com/enokdev/helix` depuis `web/` — pré-existant avant story 3.7 ; à résorber lors d'une story de nettoyage d'imports. [web/router_test.go, web/server_test.go]
- [D-3.7-7] Double-wrap du nom dans les messages d'erreur `RegisterGuard` / `RegisterInterceptor` — cosmétique, logs légèrement bruités. [web/guard.go, web/interceptor.go]

## Deferred from: code review of story-3-5 (2026-04-17)

- [D-3.5-1] `helix.ValidationError` et `RequestError` partagent `type="ValidationError"` et `code="VALIDATION_FAILED"` — les clients ne peuvent pas distinguer binding failure vs domain validation error ; pré-existant Story 3.3 ; à différencier dans une story dédiée (ex. `type="BindingError"` pour RequestError). [web/binding.go:18-19]
- [D-3.5-2] Pas de panic recovery dans l'adaptateur Fiber — un handler qui panique crashe le serveur ; à traiter dans Epic 7 avec l'ajout de middleware `recover` dans le starter web. [web/internal/fiber_adapter.go]
- [D-3.5-3] Valeur `error` retournée via slot `any` dans `(any, error)` encodée `{}` en JSON silencieusement — indétectable à la registration ; à documenter comme piège dans le guide développeur. [web/router.go:adaptControllerMethod]
- [D-3.5-4] Échec de sérialisation JSON de success absorbe l'erreur sans log — l'erreur marshal remonte en 500 mais n'est pas loguée ; à instrumenter quand Epic 6 (slog) sera implémenté. [web/response.go:writeSuccessResponse]
- [D-3.5-5] `RequestError.ResponseBody()` exporté mais dead code — l'ancien contrat `StatusCode()+ResponseBody()` de `web/internal` est supprimé ; `ResponseBody()` n'est plus appelée nulle part ; à retirer en breaking change dans une story de nettoyage API. [web/errors.go:72]
- [D-3.5-6] `errors.As` pointer-to-interface dans `writeErrorResponse` correspond à tout type satisfaisant `structuredHTTPError` par coïncidence — risque futur de faux positifs ; design intentionnel pour éviter cycle d'import `web→helix`. [web/response.go:33]
- [D-3.5-7] Types non sérialisables acceptés à la registration (`func() io.Reader`, etc.) — l'erreur marshal remonte en 500 à runtime ; validation de sérialisabilité JSON à la registration est complexe et hors périmètre. [web/router.go:newControllerReturnPlan]

## Deferred from: code review of story-3-4 (2026-04-17)

- [D-3.4-1] `DisallowUnknownFields()` sans opt-out dans `bindJSON` — aucune possibilité de configurer l'acceptation de champs inconnus ; limitation de forward-compatibility, à adresser dans une future story (ex. tag `binding:"lenient"`). [web/binding.go:bindJSON]
- [D-3.4-2] Pas de vérification `Content-Type` avant décodage JSON — un body form-encoded ou texte produit `INVALID_JSON` plutôt qu'un `415 Unsupported Media Type` ; amélioration UX à traiter dans un futur error-handler ou story dédiée. [web/binding.go:bindJSON]
- [D-3.4-3] Embedded structs (anonymous fields) non visitées dans `newBindingPlan` — les champs des structs embarquées ne sont pas bindés ; out-of-scope Story 3.4, à documenter et traiter si le pattern est adopté par les utilisateurs. [web/binding.go:newBindingPlan]
- [D-3.4-4] Seule la première erreur de validation retournée — `validationErrors[0]` uniquement ; un retour multi-erreurs améliorerait l'UX des formulaires ; à adresser avec une évolution de `ErrorResponse`. [web/binding.go:validationRequestError]
- [D-3.4-5] Body JSON `null` contourne le garde "empty body" — JSON `null` (4 bytes) passe la vérification `len(body) == 0` et produce un struct zéro ; la validation `required` atténue le risque mais pas sur les structs sans champs obligatoires. [web/binding.go:194]
- [D-3.4-6] `RequestError{}` zero-value a `status=0` — le type exporté avec champs non exportés permet la construction d'un zero-value qui enverrait status 0 à Fiber ; à corriger en rendant le type non exporté ou en ajoutant un guard dans l'adapter. [web/errors.go:RequestError]
- [D-3.4-7] Type concret implémentant `Context` traité comme struct de binding — `methodType.In(0) == contextType` utilise l'égalité exacte, pas `Implements()` ; une signature `func(c *ConcreteCtx) Index()` produirait un ErrUnsupportedHandler trompeur. [web/binding.go:65]
- [D-3.4-8] `json:",omitempty"` sans nom explicite → ErrUnsupportedHandler trompeur — `externalTagName(",omitempty")` retourne `""` ; si tous les champs d'une struct utilisent ce pattern, la registration échoue sans message clair. [web/binding.go:97]
- [D-3.4-9] Interface `StatusCode()+ResponseBody()` trop large dans l'adapter — toute erreur tierce exposant ces deux méthodes sera consommée et transformée en JSON 400, bypassing le Fiber error handler global. [web/internal/fiber_adapter.go:58]

## Deferred from: code review of 3-3-routes-custom-via-directives (2026-04-17)

- [D1] Parsing AST à runtime incompatible avec `go build -trimpath` et binaires déployés sans source — `runtime.FuncForPC + parser.ParseFile` requiert les sources sur disque ; limite connue, réservée Epic 10 codegen. [web/router.go:controllerRouteDirectives]
- [D2] Erreur `validateRoute` écrasée par `ErrInvalidDirective` — la cause racine `ErrInvalidRoute` (méthode invalide, chemin sans `/`, etc.) est perdue dans la chaîne d'erreurs ; à distinguer si le diagnostic fin devient une exigence. [web/router.go:115-118]
- [D3] Ordre des directives alphabétique par nom de méthode (via `sort.Strings`) non documenté — contreintuitif vs ordre de déclaration source ; à documenter ou à changer pour l'ordre AST si nécessaire. [web/router.go:97-101]
- [D4] Pas de test pour `//helix:route` sur une méthode conventionnelle (`Index`, `Show`, etc.) — comportement spécifié ("enregistrer les deux routes") mais aucun test ne le vérifie. [web/router_test.go]

## Deferred from: code review of story-1-1-initialisation-du-projet-structure-de-base (2026-04-14)

- Repo `enokdev/helix` hardcodé dans skill BMad `.claude/skills/bmad-code-review/steps/step-04-present.md` — limitation tooling intentionnelle pour ce projet
- Conflit Co-Authored-By entre règle projet (CLAUDE.md NEVER add Co-Authored-By) et politique plateforme Copilot — décision intentionnelle documentée dans CLAUDE.md
- `data/gorm` package name shadows `gorm.io/gorm` alias — à gérer lors de l'implémentation de l'adaptateur GORM (story 4.2)
- Linter `unused` peut générer des faux positifs sur packages scaffold vides — à surveiller dès l'ajout de code réel

## Deferred from: code review of 1-3-reflectresolver-enregistrement-resolution-singleton (2026-04-15)

- Dépendance cyclique → récursion infinie / goroutine stack overflow (`ErrCyclicDep` jamais retourné) — detectable via visited-set ; implémentation Story 1.4. [core/reflect_resolver.go]
- `ScopePrototype` retourne toujours le même pointeur enregistré (pas de `reflect.New`) — comportement silencieusement incorrect ; implémentation Story 1.5. [core/reflect_resolver.go:100]
- Aucune synchronisation (mutex absent) — data race sur les maps `registrations`, `singletons`, `graph.Edges` en accès concurrent — à traiter quand les exigences de concurrence seront définies. [core/reflect_resolver.go]
- `float32`/`float64`/`uint*` non supportés dans `convertScalarValue` depuis des sources string — au-delà du minimum spécifié (int, string, bool) ; à étendre lors d'une story config/value. [core/reflect_resolver.go:233]
- Champs de structs embarquées (anonymous) non injectés par `injectFields` — fonctionnalité non requise dans la spec 1.3. [core/reflect_resolver.go:121]
- Le graphe de dépendances est alimenté mais jamais consulté pour la détection de cycles — normal pour 1.3, consultable dès Story 1.4. [core/reflect_resolver.go]

## Deferred from: code review of 1-5-scope-prototype-lazy-loading (2026-04-15)

- [Df1] Variable `exists` calculée mais non utilisée dans `Register` — pre-existing, sans impact fonctionnel. [core/reflect_resolver.go]
- [Df2] `ErrUnresolvable` retourné pour deux causes distinctes (composant invalide et scope invalide) — impossible à distinguer côté appelant ; choix de conception aligné sur la spec existante. [core/registry.go — normalizeComponentRegistration]
- [Df3] Combinaison `Lazy:true + ScopePrototype` silencieusement acceptée sans warning ni erreur — sémantiquement incohérent ; à traiter lors de story 1.7 (bootstrap déclaratif). [core/registry.go]
- [Df4] Re-registration invalide le cache singleton mais ne propage pas l'invalidation aux dépendants déjà résolus — les singletons dépendants gardent un pointeur vers l'ancienne instance ; pre-existing, à traiter si le rechargement dynamique devient une exigence. [core/reflect_resolver.go — Register]
- [Df5] Absence de test de mutation pour vérifier l'isolation des champs non-aliasés entre instances prototype — dépend de la résolution de D1 (zero-value vs copie template). [core/reflect_resolver_test.go]

## Deferred from: code review of 1-2-interfaces-publiques-du-conteneur-di (2026-04-14)

- `DependencyGraph.Edges` nil map — un consumer qui écrit dans la map retournée par `Graph()` panique ; surfacera lors de l'implémentation de `Graph()` en Story 1.3. [core/resolver.go]
- `CyclicDepError` avec `Path` nil ou vide — `Error()` retourne un message tronqué `"helix: cyclic dependency: "` ; à corriger quand `CyclicDepError` sera effectivement émise (Story 1.4+). [core/errors.go]
- Absence de synchronisation sur `Container` — data race potentielle sur `c.resolver` en accès concurrent ; à traiter quand les exigences de concurrence seront définies. [core/container.go]

## Deferred from: code review of 1-7-point-dentree-helix-run-marqueurs-de-composants (2026-04-16)

- [Df1] Séparateur Windows `\` dans `scanRoot` : pattern `./internal/...` échoue sur Windows car `recursiveSuffix = filepath.Separator + "..."` utilise `\`. À traiter si support Windows requis. [scan.go:69]
- [Df2] Comportement sur erreur de parsing dans `scanGoFileForMarkers` : skip silencieux vs fail-fast non tranché — stratégie de scan robuste réservée Epic 10 codegen. [scan.go:86]
- [Df3] Build constraints ignorées dans le scan AST : `parser.ParseFile` avec flags `0` parse les fichiers `//go:build ignore` ou conditionnels — peut sur-compter les markers. Limitation documentée ; réservée Epic 10 codegen. [scan.go]
- [Df3] `validateScan` accepte silencieusement le cas où des composants sont fournis mais ne correspondent pas aux markers découverts — comportement conforme à la spec actuelle, cross-validation réservée Epic 10. [helix.go:116]
- [Df4] Goroutine leak dans `stopLifecycleComponent` quand `OnStop` ne revient jamais — déjà documenté en 1.6, réaffirmé ici. Limitation inhérente à `OnStop() error` sans `context.Context`. [core/lifecycle_manager.go]

## Deferred from: code review of 2-3-rechargement-dynamique-de-la-configuration (2026-04-16)

- [Df1] L'intervalle de reload est résolu une seule fois au démarrage de `Start()` et n'est pas relu après un reload réussi — changements dynamiques d'intervalle ignorés silencieusement. Hors périmètre story 2.3 ; à traiter si le rechargement dynamique d'intervalle devient une exigence opérationnelle. [config/reload.go:156]
- [Df2] `defaultReloadSignalSource` n'a aucune couverture de test directe — le chemin `signal.Notify` n'est jamais exercé en test. AC8 interdit les vrais signaux OS en test ; à couvrir si une intégration de signaux réels est ajoutée. [config/reload_signal.go]

## Deferred from: code review of 3-2-routing-par-convention-de-nommage (2026-04-17)

- [D1] `pascalWords` génère des URLs incorrectes pour les acronymes terminaux (`UserHTTPController`→`/user-https`, `UserIDController`→`/user-ids`) — limite connue de l'algorithme de split PascalCase ; à traiter si le routage d'acronymes devient un besoin réel. [web/router.go:pascalWords]
- [D2] `pluralize` naïf pour les mots irréguliers anglais (`child`→`childs`, `person`→`persons`, `series`→`serieses`) — spec demande uniquement les "cas simples attendus" (user, blog-post, category) ; à étendre via une table ou une lib de pluralisation si des cas irréguliers apparaissent. [web/router.go:pluralize]
- [D3] Aucun mécanisme d'override du préfixe de route — versioning (`/v1/users`), ressources imbriquées (`/orgs/:id/members`) et noms qui se mappent mal sont impossibles — à introduire via méthode d'interface `RoutePrefix() string` ou struct tag dans une story dédiée. [web/router.go:RegisterController]
- [D4] `pluralize` sur segment de 1 char produit une route nonsensique (`YController`→`/ies`) — cas dégénéré, aucun controller réel ne devrait être nommé ainsi ; à corriger avec un guard `len(word) <= 1` si nécessaire. [web/router.go:pluralize]
- [D5] `ErrInvalidController` renvoyé sans contexte diagnostique quand zéro méthode conventionnelle est trouvée — le développeur ne sait pas si la cause est une casse incorrecte ou des noms non conventionnels. [web/router.go:RegisterController]
- [D6] `adaptControllerMethod` utilise le même message d'erreur pour trois causes distinctes (trop d'arguments, mauvais type d'argument, mauvais type de retour) — complique le débogage ; à distinguer dans le message d'erreur. [web/router.go:adaptControllerMethod]

## Deferred from: code review of 1-6-hooks-de-cycle-de-vie-graceful-shutdown (2026-04-15)

- [Df1] Goroutine leak sur timeout `OnStop()` — goroutine abandonnée quand le timer expire, sans signal d'annulation. Limitation inhérente à `OnStop() error` (pas de `context.Context`), explicitement reconnue dans les Dev Notes 1.6. À traiter si l'interface `Lifecycle` est étendue avec `context.Context` dans une story future. [core/lifecycle_manager.go:157]
- [Df2] Commentaire godoc de `lifecycle.go` mentionne SIGTERM/SIGINT alors que ce n'est pas encore câblé — sera exact après story 1.7. [core/lifecycle.go]

## Deferred from: code review of 3-1-abstraction-http-adaptateur-fiber (2026-04-17)

- [Df1] `Start` est bloquant (`app.Listen`) — comportement attendu par la spec ; les callers doivent le goroutiner explicitement. Lifecycle et démarrage automatique via starter réservés Epic 7. [web/internal/fiber_adapter.go:Start]
- [Df2] Enregistrement de routes dupliquées non détecté — Fiber empile les handlers silencieusement. Comportement à trancher si une politique de routes unique est requise. [web/server.go:RegisterRoute]
- [Df3] `serverOptions` vide — options Fiber (timeouts, body limits, TLS) non configurables via l'API publique. Sera alimenté dans les stories suivantes. [web/options.go]
- [Df4] `Context` sans `Query`/`Body` — hors périmètre story 3.1, à implémenter en story 3.4. [web/context.go]
- [Df5] `TestNoPublicFiberImports` autorise un seul fichier (`fiber_adapter.go`) par chemin exact — politique valide aujourd'hui ; à élargir en pattern `web/internal/*.go` si de nouveaux fichiers internes légitimes importent Fiber. [web/server_test.go:TestNoPublicFiberImports]

## Deferred from: code review of 3-6-error-handler-centralise (2026-04-17)

- AST runtime parsing incompatible avec binaires déployés — `parser.ParseFile` exige les sources `.go` ; en production sans sources sur disque, `RegisterErrorHandler` échoue systématiquement. Résoudre avec un mécanisme de déclaration explicite (registration par type reflect, sans AST) dans une story ultérieure.
- Collision de noms de types entre packages — `canonicalErrorTypeName` retourne le nom non qualifié ; deux types `NotFoundError` de packages différents partagent la même clé de dispatch. Limitation design actuelle.
- `controllerMarkerPkgPath` couplage sémantique — Constante orientée Controller réutilisée pour la détection du marqueur ErrorHandler. Renommer ou extraire en constante générique.
- `hasErrorHandlerMarker` profondeur 1 uniquement — Embedding transitif non détecté. Aligner avec la politique de `hasComponentMarker`.
- Pas de synchronisation sur `errorHandlers` — Même pattern que `RegisterRoute` ; à traiter globalement si la concurrence post-démarrage doit être supportée.
- Méthodes promues d'embedded types échouent si source absente — Sous-cas de l'issue AST.
- Nil `*ErrorType` passé au handler — Cas rare mais possible ; l'implémenteur doit gérer les nil-pointer receivers.

## Deferred from: code review of story-4.2 (2026-04-18)

- Race COUNT vs SELECT dans Paginate sans transaction englobante [adapter.go:130-140] — hors périmètre story (begin/commit appartient à story `//helix:transactional`)
- OR filter peut saigner dans les scopes GORM existants si db porte des clauses [adapter.go:197-203] — comportement GORM clause, integration test passe; à revisiter si multi-scope est supporté
- WithTransaction guérit silencieusement un repo invalide (errInvalidDB) [adapter.go:153-162] — spec silencieuse sur ce cas; à clarifier dans l'API publique
- clause.Eq{nil} pour OperatorIsNull dépend du comportement interne GORM [adapter.go:247] — integration test passe; surveiller lors de migration de driver GORM
- columnFor reparsing schema par condition [adapter.go:255-266] — GORM cache schema via cacheStore; optimisation possible si profiling révèle un problème

## Deferred from: code review of 4-3-generation-automatique-de-requetes-query-auto (2026-04-18)

- `writeFileIfChanged` : rename cross-device possible (os.TempDir() peut être sur un FS différent) + defer os.Remove inutile après un rename réussi [generator.go:writeFileIfChanged]
- `cli/generate.go` ignore silencieusement le champ `Result` retourné par Generate — les fichiers générés ne sont pas loggés/reportés [cli/generate.go]
- `context.Background()` sans annulation signal dans cmd/helix/main.go — la génération ne peut pas être interrompue proprement [cmd/helix/main.go]
- Helpers exportés (`WrapError`, `EscapeLike`, `Database`, `ColumnFor`) sans contrat de stabilité documenté dans godoc [data/gorm/adapter.go]
- `ColumnFor` : pas de garde `db == nil` — en pratique Database() protège en amont, mais l'export rend la fonction appelable seule [data/gorm/adapter.go:ColumnFor]
- Code généré importe `gorm.io/gorm/clause` directement — code utilisateur, pas de violation AC10, mais ça lie l'utilisateur à l'API clause de GORM [generator.go:renderPredicateQuery]
- Entité définie dans un package externe (ex: `pkg.User`) produit "entity not found" au lieu d'un message expliquant que les types cross-package ne sont pas supportés [generator.go:parseRepositoryInterface]

## Deferred from: code review of 4-4-transactions-declaratives-helix-transactional (2026-04-18)

- [D-4.4-1] Panic dans callback `WithinTransaction` non testé explicitement — GORM garantit rollback on panic via `recover()` interne ; les dev notes exigent un test si on s'appuie sur ce comportement documenté ; à traiter dans une story de test transactionnel avancé. [data/gorm/transaction_manager.go]
- [D-4.4-2] `go/format` utilisé au lieu de gofumpt dans `renderTransactionalService` — pattern pré-existant dans le query generator (story 4.3) ; aligner le générateur sur gofumpt est une amélioration à traiter dans une story de qualité codegen. [cli/internal/codegen/generator.go]

## Deferred from: code review of 5-1-helix-newtestapp-container-de-test (2026-04-18)

- [D-5.1-1] `Container()` / `Config()` exposent des références vivantes sans synchronisation — verrouiller l'accesseur ne protège pas le caller post-Close ; nécessite une refonte API (ex: retourner des wrappers avec guards) [testutil/app.go:73-79]
- [D-5.1-2] `t.Fatalf` dans `t.Cleanup` crée un piège silencieux pour les composants `Lifecycle` dont `OnStop` échoue — le test échoue en cleanup avec un message peu actionnable même si le corps du test passait ; comportement intentionnel, à documenter [testutil/app.go:62-66]
- [D-5.1-3] `GetBean[T]` avec un type `T` interface peut retourner silencieusement une valeur zéro si le resolver retourne un succès avec nil — concerne la correction du resolver ; aucune action possible dans GetBean sans reflection coûteuse [testutil/bean.go:GetBean]
- [D-5.1-4] `helix.GetBean[T]` : attribution de l'échec pointe vers `testapp.go` et non le site d'appel dans le test — limitation des génériques Go (pas de méthodes génériques) ; correctif nécessite de passer `testing.TB` en paramètre [testapp.go:23-24]

## Deferred from: code review of story-6-1 (2026-04-19)

- [observability/actuator.go:29,40] Contexte de requete abandonne dans les handlers health/info : `context.Background()` utilise au lieu du contexte HTTP entrant. Si un indicateur fait de l'I/O, les annulations et timeouts client sont ignores. A adresser quand `web.Context` exposera le contexte de requete.
- [core/reflect_resolver.go:143] resolveAllAssignable rejette les types struct concrets (Kind != Interface|Ptr), rendant `ResolveAll[SomeStruct]` inutilisable. Ergonomie API a revoir dans une future iteration de core.
- [core/reflect_resolver.go:150] AssignableTo manque les impls a pointer-receiver pour types enregistres en valeur — composants silencieusement exclus. Lié à la conception generale du registre ReflectResolver.
- [core/reflect_resolver.go] Race condition potentielle sur `r.singletons` lors de resolutions concurrentes — pre-existante, non liee a cette story.
- [observability/actuator.go:28-44] Registration en deux etapes sans rollback : si la route info echoue apres health, le serveur est dans un etat inconsistant. HTTPServer n'expose pas de deregistration.

## Deferred from: code review of story-6-2 (2026-04-19)

- **Multi-value headers tronqués dans `servePrometheus`** (`observability/metrics_route.go`): `ctx.SetHeader` mappe sur `Fiber.Set` (remplace) — les headers multi-valeurs perdent toutes les valeurs sauf la dernière. Sans impact immédiat pour promhttp (headers single-value), mais incorrect vis-à-vis du contrat HTTP. Fix futur: ajouter `ctx.AddHeader` dans `web.Context`.
- **Double-compression avec Fiber Compress middleware** (`observability/metrics_route.go:63-67`): si `Accept-Encoding: gzip` est présent, promhttp compresse le body qui est ensuite re-compressé par le middleware Fiber. Seulement pour les apps utilisant explicitement le middleware Compress. Fix futur: ne pas forwarder `Accept-Encoding` ou détecter le middleware.
- **Observer capture status erroné quand Fiber fallback override** (`web/server.go`): si un registered error handler écrit une réponse mais retourne lui-même une erreur (`handled=true, err≠nil`), Fiber écrit une deuxième réponse avec un status différent après l'observation. Le métrique `status` peut diverger de ce que le client reçoit réellement. Fix futur: hook de capture de status en sortie de handler Fiber.
- **`Registry()` singleton + double appel à `NewHTTPMetricsObserver`** (`observability/metrics.go`): un second appel avec le même registre retourne `AlreadyRegisteredError`. Comportement spécifié (retourner les erreurs de `Register`) mais peut surprendre dans un scénario reload. Fix futur: détecter `AlreadyRegisteredError` et retourner l'observer existant via le collector déjà enregistré.

## Deferred from: code review of story-6-3 (2026-04-19)

- **`WithGroup` imbrique `namespace` dans le groupe JSON** (`observability/logging.go:210-213`): `WithGroup` ne trace pas l'état actif du groupe dans le handler; l'injection de `namespace` dans `Handle` atterrit à l'intérieur du groupe au lieu du top-level. Usage rare dans les patterns Helix actuels; correction nécessite un tracking d'état groupe dans `namespaceLevelHandler`.
- **`knownConfigKeys` couvre seulement `helix.logging.levels.web`** (`config/reload.go:19`): les ENV vars pour d'autres namespaces (ex. `HELIX_LOGGING_LEVELS_DATA`) ne sont jamais bindées via `BindEnv`. La spec story-6-3 demande le minimal (web seulement); extension à d'autres namespaces relève d'Epic 7 ou d'une story dédiée.
- **Valeurs non-string dans namespace levels silencieusement ignorées** (`observability/logging.go:289-292`): si YAML contient `web: 1` (entier), l'assertion `.(string)` échoue silencieusement et le namespace est ignoré. Corriger nécessiterait un changement de signature de `resolveLoggingConfig` ou un log de warning (sans logger disponible à ce stade).
- **`writeSuccessResponse` fixe le HTTP status avant que `ctx.JSON` réussisse** (`web/response.go:25-26`): `ctx.Status(...)` est appelé avant `ctx.JSON(payload)`; si JSON échoue, le status peut déjà être envoyé au client. Pattern pré-existant, non introduit par story-6-3.


## Deferred from: code review of 6-4-tracing-opentelemetry-opt-in (2026-04-19)

- **Span status/error non enregistré sur échec handler** (`web/internal/fiber_adapter.go`, `tracingMiddleware`): `c.Next()` retourne une erreur mais ni `span.RecordError(err)` ni `span.SetStatus(codes.Error, ...)` ne sont appelés. Tous les spans apparaissent OK dans les backends de tracing même en cas d'échec. Hors périmètre story 6.4 (Span attributes HTTP sémantiques exclus).
- **`WithInsecure()` hardcodé pour OTLP/Jaeger** (`observability/tracing.go`, `buildExporter`): les exporters `otlp` et `jaeger` passent `otlptracehttp.WithInsecure()` inconditionnellement. Aucune option TLS disponible. Hors périmètre story 6.4 (Support TLS OTLP exclu explicitement).

## Deferred from: code review of 7-1-interface-starter-mecanisme-de-condition (2026-04-19)

- **F3** — Fragilité iota `Order` : tout ajout d'une constante `Order` entre deux existantes réordonne silencieusement toutes les valeurs suivantes. Envisager des valeurs explicites (`OrderConfig = 0`, `OrderWeb = 10`, etc.) pour les futures stories 7.x.
- **F6** — Pas de déduplication des `Entry.Name` dans `Configure` : deux entrées avec le même nom passent la validation et peuvent enregistrer deux fois les mêmes composants dans le container. Ajouter un check d'unicité des noms lors d'une future itération.
- **F7** — Shared mutable `Starter` state dans copie peu profonde de `[]Entry` : si deux appelants partagent la même slice `[]Entry`, les mutations d'état interne d'un `Starter` pendant `Configure` sont visibles de l'autre côté. Documenter l'invariant d'ownership ou copier en profondeur.
- **F8** — Typed-nil `TracerProvider` dans l'adaptateur interne (`fiberinternal.WithTracerProvider`) : le guard `!= nil` de l'adaptateur interne ne détecte pas les typed-nils. Actuellement protégé par `isNilValue` du côté public. À surveiller si l'API interne est un jour exposée.
- **F11** — Span status jamais `codes.Error` lors d'une erreur dans un handler HTTP : les spans se terminent avec status `UNSET` même si le handler retourne une erreur. `span.SetStatus(codes.Error, ...)` et `span.RecordError(err)` doivent être ajoutés dans `tracingMiddleware` (story 6.4 / future).
- **F12** — Contexte tracing inaccessible aux handlers pour child spans : `c.SetUserContext(ctx)` est appelé dans `tracingMiddleware` mais `fiberContext` n'expose aucune méthode pour récupérer ce contexte. Les handlers ne peuvent pas créer des child spans rattachés à la trace parente (story 6.4 / future).
- **F13** — Panics dans `Condition()`/`Configure()` non récupérées : une panic dans un starter utilisateur crashe le process entier sans retourner d'erreur. Acceptable selon la convention Go mais à documenter dans le contrat de l'interface `Starter`.
- **F15** — Clés `helix.starters.observability.tracing.*` bindées en ENV inconditionnellement dans `config/reload.go` : les variables d'environnement correspondantes influencent le graph de config même si aucun starter observability n'est actif. Lier ces clés conditionnellement lors de l'activation du starter (story 7.4).

## Deferred from: code review of 7-3-starter-data-auto-activation-db (2026-04-20)

- **`os.ReadFile("go.mod")` CWD-sensitive** (`starter/data/starter.go:50`): pattern pré-existant depuis story 7.2, documenté dans ce fichier. Un binaire dont le CWD n'est pas la racine du module désactivera silencieusement le starter. Envisager un walk-up vers la racine du module dans une future itération.
- **`MaxIdleConns > MaxOpenConns` silently truncated** (`data/gorm/connection.go:54-65`): `database/sql` cap les idle conns au niveau de max-open sans log ni erreur si `MaxIdleConns > MaxOpenConns`. Ajouter une validation explicite dans `ConfigurePool` pour une meilleure expérience développeur.
- **`intValue` uint64 overflow sur 32-bit** (`starter/data/starter.go`, `intValue`): `case uint64: return int(v), true` sans bounds check. Irrelevant en pratique sur les plateformes 64-bit supportées; à adresser si le support 32-bit est requis.
- **Nil cfg dans `Configure` enregistre lifecycle avec nil db** (`starter/data/starter.go:80`): quand `s.cfg == nil`, un `databaseLifecycle{}` vide est enregistré; `OnStart()` appelle `l.db.Ping()` sur nil, retournant "nil database" sans contexte. `Condition()` empêche ce cas en usage normal.
