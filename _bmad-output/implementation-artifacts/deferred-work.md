
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

## Deferred from: code review of 1-6-hooks-de-cycle-de-vie-graceful-shutdown (2026-04-15)

- [Df1] Goroutine leak sur timeout `OnStop()` — goroutine abandonnée quand le timer expire, sans signal d'annulation. Limitation inhérente à `OnStop() error` (pas de `context.Context`), explicitement reconnue dans les Dev Notes 1.6. À traiter si l'interface `Lifecycle` est étendue avec `context.Context` dans une story future. [core/lifecycle_manager.go:157]
- [Df2] Commentaire godoc de `lifecycle.go` mentionne SIGTERM/SIGINT alors que ce n'est pas encore câblé — sera exact après story 1.7. [core/lifecycle.go]

## Deferred from: code review of 3-1-abstraction-http-adaptateur-fiber (2026-04-17)

- [Df1] `Start` est bloquant (`app.Listen`) — comportement attendu par la spec ; les callers doivent le goroutiner explicitement. Lifecycle et démarrage automatique via starter réservés Epic 7. [web/internal/fiber_adapter.go:Start]
- [Df2] Enregistrement de routes dupliquées non détecté — Fiber empile les handlers silencieusement. Comportement à trancher si une politique de routes unique est requise. [web/server.go:RegisterRoute]
- [Df3] `serverOptions` vide — options Fiber (timeouts, body limits, TLS) non configurables via l'API publique. Sera alimenté dans les stories suivantes. [web/options.go]
- [Df4] `Context` sans `Query`/`Body` — hors périmètre story 3.1, à implémenter en story 3.4. [web/context.go]
- [Df5] `TestNoPublicFiberImports` autorise un seul fichier (`fiber_adapter.go`) par chemin exact — politique valide aujourd'hui ; à élargir en pattern `web/internal/*.go` si de nouveaux fichiers internes légitimes importent Fiber. [web/server_test.go:TestNoPublicFiberImports]
