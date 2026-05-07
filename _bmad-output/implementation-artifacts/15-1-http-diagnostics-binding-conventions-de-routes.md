# Story 15.1: HTTP — Diagnostics, Binding & Conventions de Routes

Status: in-progress

## Story

En tant que développeur exposant une API Helix,
je veux des erreurs HTTP plus précises et des conventions de routing configurables,
afin de diagnostiquer rapidement les problèmes de binding, validation et découverte de routes.

## Acceptance Criteria

1. Une erreur de binding JSON retourne un type/code distinct d'une validation métier : `BindingError` + code adapté pour les erreurs de binding, `ValidationError` + `VALIDATION_FAILED` pour les validations applicatives.
2. Plusieurs erreurs de validation dans un body JSON ou une query sont toutes retournées dans un ordre stable, avec le format `{"errors":[{"field":"...","msg":"..."}]}` réellement émis par la réponse HTTP.
3. Les query params supportent les floats (`float32`, `float64`) et les slices simples via valeur comma-separated (`[]string`, `[]int*`, `[]uint*`, `[]float*`, `[]bool`) ou retournent une erreur `INVALID_QUERY_PARAM` qui nomme le champ et le type non supporté.
4. `UserHTTPController` et `UserIDController` ne génèrent jamais `/user-https` ou `/user-ids`; les acronymes techniques terminaux redondants sont retirés avant pluralisation.
5. Un override explicite du préfixe de route via embed `helix.Controller` taggé `helix:"route:/v1/users"` est supporté, validé et couvert par des tests qui vérifient la route enregistrée, pas seulement l'absence d'erreur.
6. Les erreurs de `RegisterController` incluent le type du controller, le handler concerné quand applicable, et la cause précise (`ErrInvalidController`, `ErrUnsupportedHandler`, `ErrInvalidDirective`, ou erreur de route wrappée avec `%w`).

## Tasks / Subtasks

- [ ] Corriger le modèle d'erreur de binding (AC: 1, 2)
  - [ ] Introduire des constantes séparées pour `BindingError` et `ValidationError` dans `web/binding.go` / `web/errors.go`.
  - [ ] Garder `helix.ValidationError` inchangé : il doit rester une validation métier avec `ErrorType() == "ValidationError"`.
  - [ ] Faire en sorte que `writeErrorResponse` respecte le body spécifique d'un `RequestError`, y compris `ValidationErrorResponse`, au lieu de reconstruire systématiquement `ErrorResponse`.
  - [ ] Conserver `errors.Is(err, web.ErrInvalidRequest)` pour tous les `RequestError`.
- [ ] Finaliser les erreurs multiples et leur couverture HTTP end-to-end (AC: 2)
  - [ ] Ajouter un test via `web.RegisterController` + `ServeHTTP` qui vérifie le JSON `errors` sur plusieurs erreurs de body.
  - [ ] Ajouter un test équivalent pour plusieurs erreurs de query.
  - [ ] Vérifier l'ordre des erreurs selon l'ordre des champs déclarés, y compris avec embedded structs.
- [ ] Étendre le binding query (AC: 3)
  - [ ] Ajouter `float32` / `float64` et pointeurs vers floats dans `isSupportedQueryField`, `isNumericField`, `validateMaxTagValue`, `exceedsMax`, `setQueryValue`.
  - [ ] Ajouter les slices simples en parsing comma-separated sans modifier `web.Context`; les repeated query params sont hors périmètre tant que `Context` n'expose que `Query(key) string`.
  - [ ] Refuser les slices multidimensionnelles, maps, structs, `time.Time`, et types custom non explicitement supportés avec une erreur actionnable.
  - [ ] Tester erreurs `ErrSyntax` / `ErrRange` de parsing comme `INVALID_QUERY_PARAM`.
- [ ] Durcir les conventions de route (AC: 4, 5)
  - [ ] Vérifier que les tests actuels couvrent `UserIDController`; ajouter le cas si absent.
  - [ ] Faire tester l'override `helix:"route:/v1/users"` en inspectant les routes enregistrées ou via `ServeHTTP`, pas uniquement `NoError`.
  - [ ] Ne pas introduire de dépendance externe de pluralisation dans cette story; les pluriels irréguliers restent hors périmètre.
- [ ] Améliorer les diagnostics de registration (AC: 6)
  - [ ] Distinguer les causes de `adaptControllerMethod` / `newControllerArgumentPlan` / `newControllerReturnPlan` dans les messages.
  - [ ] Pour les routes invalides, préserver l'erreur racine de `validateRoute` dans la chaîne au lieu de ne retourner que `ErrInvalidDirective`.
  - [ ] Ajouter des tests sur les messages sans matcher des chaînes trop longues ou fragiles.
- [ ] Vérification
  - [ ] Lancer `gofumpt` sur les fichiers Go modifiés.
  - [ ] Lancer `go test ./web/...`.
  - [ ] Lancer `go test ./...` si les signatures publiques changent.

## Dev Notes

### État Actuel Important

- `web/binding.go` utilise actuellement `requestErrorType = "ValidationError"` pour toutes les erreurs de requête, y compris `INVALID_JSON` et `INVALID_QUERY_PARAM`. C'est la confusion principale à corriger. [Source: web/binding.go]
- `RequestError.ResponseBody()` sait déjà retourner `ValidationErrorResponse{Errors: ...}`, mais `writeErrorResponse` ne l'utilise pas : il reconstruit toujours une enveloppe `ErrorResponse`. Les tests unitaires de `binding_test.go` peuvent donc passer pendant que le chemin HTTP réel reste incorrect. [Source: web/errors.go; web/response.go; web/binding_test.go]
- Les anonymous fields JSON/query, le rejet de body `null`, `DisallowUnknownFields()` avec opt-out `helix:"allow-unknown"`, les acronymes terminaux, et l'override de route existent déjà partiellement. Ne pas réécrire ces zones depuis zéro; compléter et tester les gaps restants. [Source: web/binding.go; web/router.go; web/routing_acronyms_test.go; web/routing_override_test.go]
- `web.Context` expose seulement `Query(key) string`. Les repeated query params (`?tag=a&tag=b`) ne sont pas observables via l'interface actuelle; utiliser une convention comma-separated pour les slices dans cette story et documenter cette limite dans les tests. [Source: web/context.go; web/internal/fiber_adapter.go]
- `web/internal/` reste le seul endroit autorisé à importer Fiber. Toute correction de binding/routing doit rester dans `web/` sauf besoin explicite d'adapter `fiberContext`. [Source: _bmad-output/planning-artifacts/architecture.md#Abstraction HTTP; AGENTS.md]

### Fichiers Probablement Touchés

- `web/binding.go` : types d'erreurs, multi-validation, support float/slice query, validation `max`.
- `web/errors.go` : représentation de `RequestError`, enveloppes JSON, éventuelle interface interne pour body custom.
- `web/response.go` : respect du body spécifique des request errors et conservation du format structuré.
- `web/router.go` : diagnostics de signatures/routes, préservation des causes wrappées.
- `web/binding_test.go`, `web/router_test.go`, `web/routing_acronyms_test.go`, `web/routing_override_test.go`, `web/error_messages_test.go` : tests co-localisés.

### Architecture & Contraintes

- Go minimum : 1.21; ne pas utiliser d'API nécessitant une version supérieure. [Source: go.mod; AGENTS.md]
- Format des erreurs HTTP : garder une réponse structurée, jamais `{"error":"message"}` plat. [Source: _bmad-output/planning-artifacts/architecture.md#Gestion des erreurs]
- Routes conventionnelles : kebab-case, pluriel, paramètre `:id`. [Source: _bmad-output/planning-artifacts/architecture.md#Formats API & Réponses HTTP]
- Erreurs wrappées : `fmt.Errorf("web: action: %w", err)` avec contexte utile. [Source: _bmad-output/planning-artifacts/architecture.md#Gestion des erreurs]
- Tests multiples : table-driven, co-localisés dans le package concerné. [Source: _bmad-output/planning-artifacts/architecture.md#Patterns de Test]
- Pas de nouvelle dépendance pour cette story. Les conversions query doivent utiliser la stdlib (`strconv`, `strings`, `reflect`) et le validator existant.

### Garde-fous d'Implémentation

- Ne pas changer le contrat public de `helix.ValidationError`; les consommateurs peuvent déjà dépendre de `ValidationError`, `VALIDATION_FAILED`, `StatusCode()`, `ErrorField()`.
- Si une interface privée est ajoutée pour exposer un body d'erreur custom, la garder non exportée sauf nécessité claire; éviter d'élargir inutilement l'API publique.
- Pour les slices, définir explicitement le comportement des entrées vides : `tags=` doit produire une slice vide ou une erreur, mais le choix doit être testé et stable.
- Pour les floats, utiliser `strconv.ParseFloat(raw, bits)` et vérifier les erreurs de range/syntaxe; `float32` doit être assigné après parsing avec bitSize 32.
- Le `max` tag doit fonctionner pour int, uint et float; pour les slices, ne pas lui donner une sémantique ambiguë dans cette story.
- Les assertions de messages d'erreur doivent chercher des fragments stables : controller type, handler name, sentinel wrappée, cause courte. Ne pas tester des phrases complètes.
- Ne pas ajouter de texte de suivi interne dans les Go docs, README ou CONTRIBUTING; les IDs de story restent uniquement dans `_bmad-output/`.

### Informations Techniques Récentes

- `github.com/go-playground/validator/v10` documente `ValidationErrors` comme une slice de `FieldError`; l'usage actuel `validator.New(validator.WithRequiredStructEnabled())` reste aligné avec la recommandation v10 visible dans la documentation. [Source: https://pkg.go.dev/github.com/go-playground/validator/v10]
- La stdlib `strconv.ParseFloat` accepte `bitSize` 32 ou 64 et retourne un `float64` convertible vers `float32` quand `bitSize=32`; ses erreurs sont des `*strconv.NumError` avec `ErrSyntax` ou `ErrRange`. [Source: https://pkg.go.dev/strconv]
- Le projet utilise Fiber v2.52.12 mais Fiber ne doit pas apparaître hors `web/internal`. [Source: go.mod; _bmad-output/planning-artifacts/architecture.md#Abstraction HTTP]

### Historique Récent Pertinent

- Le dernier commit `fd3660b fix(web): code review patches — story 14.10` a touché `web/router.go`, `web/router_test.go`, `web/cache_interceptor.go` et des tests de DI/mock. Préserver les changements sur l'ordre guards/interceptors et l'invalidation DI; cette story ne doit pas les refactorer.
- Les stories 14.1 et 14.2 ont déjà durci panic recovery, JSON content-type, body null, embedded structs et unknown fields. Cette story doit compléter la DX, pas revenir à l'ancien comportement permissif.

### Project Structure Notes

- Les tests white-box de binding peuvent rester en package `web`; les tests API via `RegisterController` doivent rester en package `web_test` quand ils ne nécessitent pas d'accès aux symboles non exportés.
- `web/internal/fiber_adapter.go` contient `fiberContext`; n'y ajouter que ce qui est nécessaire à l'interface publique `web.Context`.
- `routing_override_test.go` définit son propre mock server; attention aux doublons avec les helpers de `router_test.go`.

### References

- `_bmad-output/planning-artifacts/epics.md` — Epic 15 / Story 15.1.
- `_bmad-output/planning-artifacts/architecture.md` — abstraction HTTP, formats API, erreurs, tests, structure package.
- `_bmad-output/implementation-artifacts/deferred-work.md` — dettes D-3.4, D-3.5, D-3.2, review 11.3.
- `web/binding.go`, `web/errors.go`, `web/response.go`, `web/router.go`, `web/context.go`, `web/internal/fiber_adapter.go`.
- `web/binding_test.go`, `web/router_test.go`, `web/routing_acronyms_test.go`, `web/routing_override_test.go`, `web/error_messages_test.go`.

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

### Completion Notes List

- Story context created from Epic 15 and current HTTP implementation analysis.
- Several ACs are already partially implemented; dev work should close gaps and strengthen end-to-end tests.

### File List
