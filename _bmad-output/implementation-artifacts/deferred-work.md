
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
