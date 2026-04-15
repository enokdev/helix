# Story 1.5: Scope Prototype & Lazy Loading

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

En tant que **développeur utilisant Helix**,
Je veux contrôler le scope d'instanciation de mes composants,
Afin d'obtenir une nouvelle instance à chaque résolution quand nécessaire.

## Acceptance Criteria

1. **Given** un composant enregistré avec `scope:"prototype"`
   **When** `container.Resolve()` est appelé deux fois
   **Then** deux instances différentes sont retournées

2. **Given** un composant configuré avec `lazy:"true"`
   **When** le container démarre
   **Then** le composant n'est pas instancié jusqu'à sa première résolution

3. **And** le Singleton reste le comportement par défaut (sans tag explicite)

## Tasks / Subtasks

- [x] Tâche 1 : Encadrer le vrai périmètre de la story dans `core/` (AC: #1 à #3)
  - [x] Garder l’API publique existante (`Container`, `Resolver`, `NewContainer`) compatible avec les stories 1.2 à 1.4
  - [x] Réutiliser `ComponentRegistration`, `Scope`, `ScopeSingleton`, `ScopePrototype` et `NewComponentRegistration()` au lieu d’introduire un second mécanisme de métadonnées
  - [x] Ne pas implémenter `helix.Run()`, le scan de packages ni les marqueurs `helix.Service`/`Controller`/`Repository`/`Component` dans cette story
  - [x] Clarifier dans le code et les tests que la configuration déclarative finale `scope:"prototype"` / `lazy:"true"` sera consommée par le bootstrap ultérieur, tandis que cette story livre d’abord la sémantique du conteneur

- [x] Tâche 2 : Ajouter un chemin explicite pour enregistrer les métadonnées de scope/lazy sans casser le comportement existant (AC: #1, #2, #3)
  - [x] Continuer à accepter l’enregistrement actuel d’un composant brut via `Register(&MyService{})`, avec défaut `singleton` et `lazy=false`
  - [x] Ajouter ou étendre le chemin d’enregistrement pour qu’un composant puisse porter explicitement `ScopePrototype` et `Lazy=true` via `ComponentRegistration` ou un helper minimal équivalent centré sur `core/`
  - [x] Valider strictement les enregistrements invalides (composant nil, non pointeur vers struct, scope vide/invalide) avec des erreurs wrappées compatibles `ErrUnresolvable`
  - [x] Préserver l’ordre déterministe de `registrationOrder` et la cohérence de `DependencyGraph`

- [x] Tâche 3 : Corriger la sémantique `prototype` dans `ReflectResolver` (AC: #1, #3)
  - [x] Faire en sorte qu’une résolution `prototype` crée une nouvelle instance via reflection à partir du type enregistré au lieu de réutiliser le pointeur source enregistré
  - [x] Réinjecter les champs `inject:"true"` et `value:"..."` sur chaque nouvelle instance prototype
  - [x] Ne jamais mettre en cache une instance `prototype` dans `singletons`
  - [x] Conserver le comportement existant pour `singleton` : même instance retournée à chaque résolution, y compris pour le composant source déjà enregistré

- [x] Tâche 4 : Formaliser le comportement `lazy` sans dériver vers le bootstrap applicatif (AC: #2)
  - [x] S’assurer qu’aucune résolution ni injection n’est déclenchée lors de l’enregistrement seul d’un composant `lazy`
  - [x] Conserver la métadonnée `Lazy` dans le registre pour que la future story 1.7 puisse distinguer les composants à pré-instancier des composants à différer
  - [x] Éviter d’ajouter une fausse API de démarrage uniquement pour “simuler” `helix.Run()` ; le contrat attendu ici est côté conteneur/résolution, pas côté signaux OS ou serveur HTTP
  - [x] Documenter dans les tests que, dans l’état actuel du projet, “instancié” signifie au minimum “résolu/injecté par le resolver”, puisque `Register()` reçoit déjà un pointeur alloué par l’appelant

- [x] Tâche 5 : Préserver les garanties existantes du resolver (AC: #1 à #3)
  - [x] Ne pas casser la détection de cycles livrée en story 1.4 pour les graphes `singleton`, `prototype` et mixtes
  - [x] Continuer d’alimenter `DependencyGraph.Edges` sans dupliquer les arêtes à chaque résolution prototype
  - [x] Garantir que les erreurs `ErrNotFound`, `ErrUnresolvable` et `ErrCyclicDep` restent observables via `errors.Is`
  - [x] Ne pas coupler `core/` au package `config/`, ni introduire de goroutine, mutex ou logique de lifecycle hors périmètre

- [x] Tâche 6 : Ajouter les tests de non-régression et de nouveaux cas `prototype`/`lazy` (AC: #1 à #3)
  - [x] Étendre `core/reflect_resolver_test.go` avec des tests table-driven pour l’enregistrement explicite des métadonnées de registration
  - [x] Vérifier qu’un composant `prototype` résolu deux fois retourne deux pointeurs différents
  - [x] Vérifier qu’un `prototype` avec dépendances reçoit bien une nouvelle instance du composant cible à chaque résolution, tout en continuant à consommer les singletons dépendants déjà enregistrés
  - [x] Vérifier qu’un composant `lazy` n’est ni injecté ni mémorisé avant la première résolution
  - [x] Vérifier que `Register(&Component{})` sans métadonnée explicite reste un singleton non lazy par défaut
  - [x] Vérifier qu’aucune entrée parasite n’est ajoutée à `singletons` pour les composants `prototype`

- [x] Tâche 7 : Valider la qualité et les contraintes d’architecture (AC: #1 à #3)
  - [x] Exécuter `go test ./core/...`
  - [x] Exécuter `go test ./...`
  - [x] Exécuter `go build ./...`
  - [x] Confirmer qu’aucun autre package Helix n’est importé depuis `core/`
  - [x] Confirmer que la story ne dérive pas vers `helix.Run()`, les hooks `Lifecycle`, le shutdown ou le chargement YAML réel

## Dev Notes

### Périmètre Strict de cette Story

La Story 1.5 doit livrer la sémantique de scope attendue dans le conteneur DI, pas le bootstrap applicatif complet.

Cette story couvre :
- la prise en charge effective de `ScopePrototype`
- la conservation exploitable de la métadonnée `Lazy`
- la compatibilité du resolver avec les comportements déjà livrés en stories 1.3 et 1.4

Hors périmètre explicite :
- `helix.Run()`, le scan de packages et les embeds marqueurs : Story 1.7
- hooks `Lifecycle`, ordre `OnStart/OnStop`, timeout de shutdown, signaux `SIGTERM/SIGINT` : Story 1.6
- chargement réel YAML/ENV/profils : Epic 2
- mode `WireResolver` / codegen compile-time : Epic 10

Si l’implémentation commence à toucher `helix.go`, `config/`, `web/` ou la gestion des signaux OS, le scope a dérapé.

### Ambiguïté Fonctionnelle à Résoudre Sans Casser l’API

Le texte des AC parle de composants enregistrés avec `scope:"prototype"` et configurés avec `lazy:"true"`, mais le code réel actuel n’a pas encore le bootstrap déclaratif qui traduira ces tags en métadonnées de résolution.

Le code déjà présent fournit cependant la brique centrale :

```go
type ComponentRegistration struct {
	Component any
	Scope     Scope
	Lazy      bool
}
```

Conséquence pratique pour cette story :
- il faut d’abord rendre la **mécanique du conteneur** correcte pour `prototype` et `lazy`
- il ne faut pas attendre la story 1.7 pour corriger le bug réel de `ScopePrototype`
- il ne faut pas inventer une API de bootstrap publique opportuniste juste pour coller littéralement au mot “démarre”

La bonne cible est donc :
- un enregistrement “simple” continue à produire un singleton non lazy
- un enregistrement “riche” peut porter `ScopePrototype` et `Lazy`
- la future story 1.7 réutilisera ces métadonnées lorsqu’elle convertira les marqueurs/tags utilisateur vers le conteneur

### Problème Réel à Corriger

Le fichier `_bmad-output/implementation-artifacts/deferred-work.md` documente explicitement le bug actuel :

- `ScopePrototype` retourne toujours le même pointeur enregistré, donc le comportement est silencieusement faux

Le code actuel de `core/reflect_resolver.go` confirme ce point :
- `resolveRegistration()` part toujours de `reflect.ValueOf(registration.Component)`
- l’instance est ensuite injectée puis potentiellement mémorisée
- aucun chemin ne crée une copie fraîche pour un composant `prototype`

Sans correction, la story 1.5 serait “validée” sur le papier tout en laissant le conteneur violer FR5.

### Stratégie d’Implémentation Recommandée

Approche recommandée :

1. Normaliser l’entrée d’enregistrement autour de `ComponentRegistration`
2. Garder `Register(&MyService{})` comme raccourci vers `NewComponentRegistration(&MyService{})`
3. Résoudre un `singleton` à partir du pointeur enregistré existant
4. Résoudre un `prototype` à partir d’une **nouvelle instance** du type concret sous-jacent

Pour `prototype`, l’algorithme attendu ressemble à ceci :
- identifier le type concret enregistré (`*MyService`)
- allouer une nouvelle valeur `reflect.New(elemType)`
- injecter récursivement ses dépendances et ses `value:"..."`
- retourner cette nouvelle instance sans la stocker dans `singletons`

Pour `singleton`, le contrat existant ne doit pas bouger :
- la première résolution peut compléter l’injection sur l’instance enregistrée
- les résolutions suivantes doivent retourner exactement le même pointeur

### Interprétation Pratique de `lazy:"true"` à ce Stade du Projet

À ce stade, `Register()` reçoit déjà un pointeur construit par l’appelant. La story 1.5 ne peut donc pas promettre “aucune allocation Go n’existe avant Resolve” si l’API d’entrée crée déjà cette allocation côté appelant.

L’interprétation utile et testable est :
- un composant `lazy` n’est pas **résolu/injecté/mis en cache** tant qu’aucune résolution explicite ne le demande
- la métadonnée `Lazy` est bien portée par le registre pour que le bootstrap futur sache quels singletons pré-instancier ou non

Important :
- ne pas ajouter une API publique de warmup artificielle juste pour tester le concept
- ne pas implémenter de pré-résolution globale dans cette story
- ne pas faire de `Lazy` une simple métadonnée morte sans test démontrant qu’aucune injection n’arrive avant la première résolution

### Garde-fous de Conception

- Ne pas déplacer `core/reflect_resolver.go` vers `core/internal/` uniquement pour coller au document d’architecture ; le code réel du projet a déjà établi `core/` comme emplacement actuel, et les stories précédentes s’appuient sur lui.
- Ne pas réécrire `Container` ou `Resolver` alors que l’API publique est déjà stabilisée.
- Ne pas casser la compatibilité `errors.Is(...)` des erreurs existantes.
- Ne pas introduire `panic()` pour signaler un scope invalide ou une registration invalide ; toujours retourner une erreur wrappée.
- Ne pas lier `core/` à `config/` pour “tester” `lazy` ou `value`.
- Ne pas introduire de synchronisation concurrente opportuniste ; les exigences de concurrence n’ont pas encore été cadrées.

### Contrat d’Intégration avec les Stories 1.6 et 1.7

Story 1.5 prépare explicitement deux histoires suivantes :

- Story 1.6 consommera l’ordre du graphe et la résolution réelle pour les hooks `Lifecycle`, mais **ne doit pas** être anticipée ici
- Story 1.7 consommera les métadonnées `Scope`/`Lazy` lorsqu’elle construira `helix.Run(App{})` et le scan déclaratif des composants

Autrement dit :
- Story 1.5 livre le moteur DI correct
- Story 1.6 livrera le cycle de vie
- Story 1.7 livrera l’ergonomie de bootstrap

### État Actuel du Resolver à Préserver

Le `ReflectResolver` actuel maintient :

```go
type ReflectResolver struct {
	registrations     map[reflect.Type]ComponentRegistration
	registrationOrder []reflect.Type
	singletons        map[reflect.Type]reflect.Value
	graph             DependencyGraph
	valueLookup       func(key string) (any, bool)
}
```

Points importants pour 1.5 :
- `registrationOrder` sert la résolution déterministe des interfaces assignables
- `singletons` ne doit contenir que des vrais singletons résolus
- `graph.Edges` ne doit pas enfler artificiellement à chaque résolution prototype répétée
- la détection de cycle introduite en 1.4 doit continuer à fonctionner même si un prototype dépend d’un singleton ou inversement

### Tests Attendus

Les tests doivent rester co-localisés dans `core/` et suivre le style déjà établi :
- table-driven tests dès qu’il y a plusieurs cas
- assertions ciblées avec `errors.Is` / `errors.As`
- validation explicite des pointeurs et de l’état interne du resolver

Cas minimaux à couvrir :
- enregistrement brut = singleton par défaut
- enregistrement explicite `prototype` = deux résolutions, deux pointeurs différents
- enregistrement explicite `lazy` = aucune injection/mise en cache avant premier `Resolve`
- dépendance injectée dans un prototype à chaque résolution sans casser le graphe
- absence de pollution de `singletons` pour les prototypes
- compatibilité avec les tests existants de cycle et de lookup interface

Éviter les faux tests qui “valident” `lazy` uniquement en lisant un booléen sans démontrer l’absence de résolution.

### Intelligence des Stories Précédentes

Apprentissages importants de 1.3 et 1.4 :
- le resolver actuel réutilise exactement l’instance enregistrée pour les singletons ; ce contrat doit survivre
- `value:"..."` est volontairement découplé de `config/` via `valueLookup` ; ne pas recoupler `core/`
- la détection de cycles repose sur un `resolutionState` synchrone ; ne pas la contourner pour implémenter `prototype`
- `deferred-work.md` signale déjà le bug prototype ; la story doit partir du code réel, pas d’un design théorique

### Git Intelligence Utile

Contexte récent :
- `f96d086 fix(core): appliquer les corrections de la code review story 1.4`
- `ae4660d feat(core): implémenter ReflectResolver — story 1.3`

Conséquence pratique :
- étendre l’implémentation existante de `ReflectResolver`
- éviter toute refonte large qui rendrait obsolètes les learnings et correctifs des deux stories précédentes

### Latest Technical Information

Aucune recherche web supplémentaire n’est nécessaire pour cette story :
- le travail attendu repose sur la stdlib Go 1.21 (`reflect`, `fmt`, `errors`)
- aucune nouvelle dépendance externe n’est requise
- le risque principal est sémantique et architectural, pas lié à une API tierce instable

### Project Structure Notes

Zone de travail attendue :

```text
core/
├── container.go
├── errors.go
├── reflect_resolver.go
├── reflect_resolver_test.go
├── registry.go
├── registry_test.go
├── resolver.go
└── ... autres fichiers existants
```

Fichiers probablement touchés :
- `core/reflect_resolver.go`
- `core/reflect_resolver_test.go`
- `core/registry.go`
- `core/registry_test.go`
- éventuellement `core/container.go` si un ajustement minimal est nécessaire pour accepter une forme enrichie d’enregistrement sans casser la signature existante

Fichiers à ne pas toucher pour satisfaire cette story :
- `helix.go`
- `config/`
- `web/`
- `data/`
- `starter/`
- `observability/`

### References

- [Source: `_bmad-output/planning-artifacts/epics.md` — Section `Story 1.5: Scope Prototype & Lazy Loading`]
- [Source: `_bmad-output/planning-artifacts/epics.md` — Sections `Story 1.6` et `Story 1.7` pour les limites de scope]
- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section `Architecture Duale DI — Couche Resolver abstraite`]
- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section `Patterns d'Implémentation & Règles de Cohérence`]
- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section `Gestion d'Erreurs`]
- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section `Frontières Architecturales`]
- [Source: `_bmad-output/planning-artifacts/architecture.md` — Section `Organisation des Tests`]
- [Source: `_bmad-output/implementation-artifacts/1-3-reflectresolver-enregistrement-resolution-singleton.md` — Sections `Dev Notes`, `Contraintes d’Architecture à Respecter`, `Intelligence de la Story Précédente`]
- [Source: `_bmad-output/implementation-artifacts/1-4-detection-des-cycles-de-dependances.md` — Sections `Dev Notes`, `Stratégie d’Implémentation Recommandée`, `État Interne du Resolver à Préserver`]
- [Source: `_bmad-output/implementation-artifacts/deferred-work.md` — entrée `ScopePrototype retourne toujours le même pointeur enregistré`]
- [Source: `core/reflect_resolver.go`]
- [Source: `core/reflect_resolver_test.go`]
- [Source: `core/registry.go`]
- [Source: `core/container.go`]
- [Source: `core/resolver.go`]

## Dev Agent Record

### Agent Model Used

GPT-5 Codex

### Debug Log References

- `sed -n '1,260p' .agents/skills/bmad-create-story/workflow.md`
- `sed -n '1,260p' _bmad-output/implementation-artifacts/sprint-status.yaml`
- `sed -n '1,260p' _bmad-output/planning-artifacts/epics.md`
- `sed -n '1,260p' _bmad-output/planning-artifacts/architecture.md`
- `sed -n '1,260p' _bmad-output/implementation-artifacts/1-3-reflectresolver-enregistrement-resolution-singleton.md`
- `sed -n '1,260p' _bmad-output/implementation-artifacts/1-4-detection-des-cycles-de-dependances.md`
- `sed -n '1,220p' _bmad-output/implementation-artifacts/deferred-work.md`
- `sed -n '1,320p' core/reflect_resolver.go`
- `sed -n '1,240p' core/registry.go`
- `sed -n '1,260p' core/reflect_resolver_test.go`
- `git log --oneline -5`
- `env GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache GOPATH=/tmp/go go test ./core/...`
- `gofmt -w core/registry.go core/reflect_resolver.go core/reflect_resolver_test.go`
- `env GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache GOPATH=/tmp/go go test ./...`
- `env GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache GOPATH=/tmp/go go build ./...`
- `golangci-lint run`
- `rg -n 'github.com/enokdev/helix|"(config|core|web|data|starter|observability|security|scheduler|testutil)(/|\\")' core`

### Completion Notes List

- Story créée automatiquement à partir de `epics.md`, `architecture.md`, `sprint-status.yaml`, des stories 1.3/1.4 et du code réel de `core/`
- Aucun `prd.md`, fichier UX, ni `project-context.md` n’a été trouvé ; le contexte s’appuie donc sur les artefacts disponibles
- Le guide de story explicite l’ambiguïté entre les tags déclaratifs futurs et la mécanique du conteneur livrable dès maintenant
- Les garde-fous couvrent explicitement les risques de régression sur les singletons, de faux “lazy loading” et de dérive prématurée vers `helix.Run()`
- Le resolver accepte désormais `ComponentRegistration` tout en conservant `Register(&Component{})` comme raccourci singleton non lazy par défaut
- `ScopePrototype` matérialise maintenant une nouvelle instance par résolution, en clonant l’état de base avant réinjection des dépendances et des valeurs
- La métadonnée `Lazy` est conservée dans le registre et les tests vérifient qu’aucun cache singleton n’est créé avant la première résolution
- Les validations exécutées et vertes sont `go test ./core/...`, `go test ./...` et `go build ./...` avec caches Go redirigés vers `/tmp` pour contourner les restrictions sandbox
- `golangci-lint` n’était pas disponible dans l’environnement (`command not found`), donc la validation lint n’a pas pu être exécutée localement dans cette session

## File List

- `core/registry.go`
- `core/reflect_resolver.go`
- `core/reflect_resolver_test.go`
- `_bmad-output/implementation-artifacts/1-5-scope-prototype-lazy-loading.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`

### Review Findings

- [x] [Review][Decision] D1 — Stratégie d'instanciation prototype : zero-value vs copie template — **Résolu : zero-value**. Suppression de `instance.Elem().Set(source.Elem())` dans `materializeRegistrationInstance`. Chaque prototype démarre avec des champs à zéro ; seuls `inject:"true"` et `value:"..."` sont peuplés. [core/reflect_resolver.go — materializeRegistrationInstance]
- [x] [Review][Decision] D2 — Lazy : métadonnée pure ou enforcement comportemental — **Résolu : métadonnée pure + test renforcé**. `Lazy` reste une métadonnée pour story 1.7. Test renforcé : assertion que `source.Dependency == nil` avant tout appel à `Resolve`. [core/reflect_resolver_test.go]
- [x] [Review][Patch] P1 — Indentation mixte (tabs vs spaces) dans `normalizeComponentRegistration` — **Corrigé via gofmt** [core/registry.go]
- [x] [Review][Patch] P2 — Test lazy manque l'assertion d'absence d'injection avant le premier Resolve — **Corrigé** [core/reflect_resolver_test.go]
- [x] [Review][Patch] P3 — Test prototype ne valide pas le comportement zero-value des champs non-inject — **Corrigé** : assertion `first.Label == "" && second.Label == ""` ajoutée [core/reflect_resolver_test.go]
- [x] [Review][Defer] Df1 — Variable `exists` calculée mais non utilisée dans `Register` [core/reflect_resolver.go] — deferred, pre-existing
- [x] [Review][Defer] Df2 — `ErrUnresolvable` retourné pour deux causes distinctes (composant invalide et scope invalide) [core/registry.go — normalizeComponentRegistration] — deferred, choix de conception aligné sur la spec
- [x] [Review][Defer] Df3 — Combinaison `Lazy:true + ScopePrototype` silencieusement acceptée sans warning [core/registry.go] — deferred, concerne story 1.7
- [x] [Review][Defer] Df4 — La re-registration ne propage pas l'invalidation aux dépendants déjà résolus [core/reflect_resolver.go — Register] — deferred, pre-existing
- [x] [Review][Defer] Df5 — Absence de test de mutation pour vérifier l'isolation des champs non-aliasés — deferred, résolu par D1 (zero-value garantit l'isolation des valeurs)

## Change Log

- 2026-04-15: ajout de la normalisation des `ComponentRegistration`, validation des scopes, support effectif des prototypes et tests de non-régression `prototype`/`lazy`, puis passage de la story en `review`
