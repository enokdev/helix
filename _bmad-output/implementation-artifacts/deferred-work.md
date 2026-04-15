
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
