# Gestion des erreurs

Helix fournit un système de gestion des erreurs en couches : mapping automatique des erreurs retournées vers des codes de statut HTTP, réponses d'erreur structurées, formatage des erreurs de validation, et gestionnaires d'erreurs de domaine centralisés.

## Comment les erreurs deviennent des réponses HTTP

Quand un handler retourne une erreur, Helix l'inspecte dans l'ordre :

1. Implémente-t-elle `web.RequestError` ? → utiliser son `StatusCode()`, sérialiser en JSON structuré
2. Est-ce une erreur de validation (produite par le binding) ? → 400 avec des détails par champ
3. Y a-t-il un `ErrorHandler` enregistré qui gère ce type ? → déléguer au handler
4. Sinon → 500 Internal Server Error (message caché en production)

```
le handler retourne une erreur
        │
        ▼
implémente RequestError ? ──oui──▶ réponse JSON avec son StatusCode
        │ non
        ▼
erreur de validation ? ──oui──▶ 400 avec tableau d'erreurs par champ
        │ non
        ▼
ErrorHandler enregistré correspond ? ──oui──▶ déléguer au handler
        │ non
        ▼
500 Internal Server Error
```

## L'interface `RequestError`

```go
type RequestError interface {
    error
    StatusCode() int    // code de statut HTTP (400, 401, 403, 404, ...)
    ErrorType() string  // nom de type lisible ("NotFound", "Unauthorized")
    ErrorCode() string  // code lisible par machine ("USER_NOT_FOUND")
    ErrorField() string // champ qui a causé l'erreur (pour les erreurs par champ)
}
```

Toute erreur implémentant cette interface est sérialisée comme :

```json
{
  "error": {
    "type":    "NotFound",
    "code":    "USER_NOT_FOUND",
    "message": "user 42 not found"
  }
}
```

## Constructeurs helpers

### `web.NewRequestError`

Erreur HTTP à usage général :

```go
// 404
return nil, web.NewRequestError(http.StatusNotFound, "utilisateur introuvable")

// 400
return nil, web.NewRequestError(http.StatusBadRequest, "paramètre de page invalide")

// 409
return nil, web.NewRequestError(http.StatusConflict, "email déjà enregistré")
```

### `web.Unauthorized` / `web.Forbidden`

```go
return nil, web.Unauthorized("token manquant ou invalide")   // 401
return nil, web.Forbidden("permissions insuffisantes")       // 403
```

### `web.NewFieldError`

Erreur liée à un champ spécifique (utile dans les services, complète la validation du binding) :

```go
return nil, web.NewFieldError("email", "cet email est déjà pris")
// → 400 avec { "error": { "type": "ValidationError", "field": "email", "message": "..." } }
```

## Erreurs de binding et de validation

Quand Helix lie le corps de la requête ou les paramètres de query et que la validation échoue, il retourne 400 automatiquement avec un tableau structuré d'erreurs par champ :

```json
{
  "errors": [
    { "type": "ValidationError", "field": "email",    "message": "doit être un email valide" },
    { "type": "ValidationError", "field": "password", "message": "longueur minimale de 8" }
  ]
}
```

Celles-ci sont produites avant que votre handler soit appelé — vous n'avez pas besoin de les gérer vous-même.

Les tags de validation proviennent de [go-playground/validator](https://github.com/go-playground/validator) :

```go
type RegisterInput struct {
    Email    string `json:"email"    validate:"required,email"`
    Password string `json:"password" validate:"required,min=8,max=72"`
    Age      int    `json:"age"      validate:"gte=18"`
}
```

## Types d'erreurs personnalisés

Définissez vos propres erreurs de domaine qui implémentent `RequestError` :

```go
type ErrOutOfStock struct {
    ProductID uint
    Requested int
    Available int
}

func (e ErrOutOfStock) Error() string {
    return fmt.Sprintf("produit %d : demandé %d mais seulement %d disponibles", e.ProductID, e.Requested, e.Available)
}

func (e ErrOutOfStock) StatusCode() int    { return http.StatusConflict }
func (e ErrOutOfStock) ErrorType() string  { return "OutOfStock" }
func (e ErrOutOfStock) ErrorCode() string  { return "PRODUCT_OUT_OF_STOCK" }
func (e ErrOutOfStock) ErrorField() string { return "" }
```

Retournez-la directement depuis n'importe quel handler ou service :

```go
func (s *OrderService) PlaceOrder(ctx context.Context, in PlaceOrderInput) (*Order, error) {
    product, _ := s.ProductRepo.FindByID(ctx, in.ProductID)
    if product.Stock < in.Quantity {
        return nil, ErrOutOfStock{
            ProductID: in.ProductID,
            Requested: in.Quantity,
            Available: product.Stock,
        }
    }
    // ...
}
```

Réponse (409 Conflict) :

```json
{
  "error": {
    "type":    "OutOfStock",
    "code":    "PRODUCT_OUT_OF_STOCK",
    "message": "produit 7 : demandé 5 mais seulement 2 disponibles"
  }
}
```

## Gestionnaires d'erreurs centralisés

Pour les erreurs que vous ne contrôlez pas (erreurs de bibliothèques tierces, `errors.New(...)`) ou quand vous voulez un seul endroit pour tous les mappings domaine → HTTP, implémentez `helix.ErrorHandler` :

```go
type DomainErrorHandler struct {
    helix.ErrorHandler
}

//helix:handles *pgconn.PgError
func (h *DomainErrorHandler) HandlePostgres(err *pgconn.PgError) (any, int) {
    switch err.Code {
    case "23505": // unique_violation
        return map[string]any{"error": map[string]any{
            "type":    "DuplicateKey",
            "message": "un enregistrement avec cette valeur existe déjà",
        }}, http.StatusConflict
    case "23503": // foreign_key_violation
        return map[string]any{"error": map[string]any{
            "type":    "ReferenceNotFound",
            "message": "l'enregistrement référencé n'existe pas",
        }}, http.StatusUnprocessableEntity
    default:
        return map[string]any{"error": "erreur base de données"}, http.StatusInternalServerError
    }
}

//helix:handles ErrOutOfStock
func (h *DomainErrorHandler) HandleOutOfStock(err ErrOutOfStock) (any, int) {
    return map[string]any{
        "error": map[string]any{
            "type":      "OutOfStock",
            "productId": err.ProductID,
            "available": err.Available,
        },
    }, http.StatusConflict
}
```

Enregistrez comme composant :

```go
helix.Run(helix.App{
    Components: []any{
        &DomainErrorHandler{},
        // ...
    },
})
```

La directive `//helix:handles TypeName` indique à Helix quel type d'erreur cette méthode gère. La signature de la méthode est :

```go
func (h *Handler) HandleXxx(err VotreTypeErreur) (corpsRéponse any, codeStatut int)
```

## Pattern domaine → HTTP

Un pattern courant est un switch unique dans la couche service qui encapsule les erreurs de domaine :

```go
func domainToHTTP(err error) error {
    switch {
    case errors.Is(err, ErrUserNotFound):
        return web.NewRequestError(http.StatusNotFound, "utilisateur introuvable")
    case errors.Is(err, ErrEmailTaken):
        return web.NewFieldError("email", "cet email est déjà enregistré")
    case errors.Is(err, ErrWeakPassword):
        return web.NewFieldError("password", "le mot de passe ne respecte pas les exigences")
    default:
        return err // laisser Helix gérer comme 500
    }
}

// Dans un handler :
func (c *UserController) Create(ctx web.Context, in CreateInput) (*User, error) {
    user, err := c.Svc.Create(ctx, in)
    if err != nil {
        return nil, domainToHTTP(err)
    }
    ctx.Status(http.StatusCreated)
    return user, nil
}
```

## Types de retour des handlers et codes de statut

| Signature de retour | Statut de succès | Gestion des erreurs |
|--------------------|-----------------|---------------------|
| `() error` | 204 No Content | standard |
| `() (T, error)` | 200 OK | standard |
| Convention méthode `Create()` | 201 Created | standard |
| Convention méthode `Delete()` | 204 No Content | standard |
| `ctx.Status(n)` explicite | n | standard |
| `(nil, nil)` | 204 No Content | pas de corps |

## Récupération des panics

Helix capture les panics dans les handlers. Un panic produit une réponse 500 sans planter le serveur :

```json
{
  "error": {
    "type":    "InternalServerError",
    "message": "une erreur inattendue s'est produite"
  }
}
```

La trace de pile complète est loguée au niveau ERROR. La valeur de panic n'est jamais exposée au client.

## Logging des erreurs

Toutes les erreurs 5xx sont loguées automatiquement :

```json
{"level":"ERROR","method":"POST","path":"/orders","status":500,"error":"sql: no rows in result set","latency_ms":12}
```

Les erreurs 4xx sont loguées au niveau DEBUG (ce sont des erreurs client attendues, pas des problèmes d'application).

## Récapitulatif

| Scénario | Que faire |
|----------|-----------|
| Introuvable | `return nil, web.NewRequestError(404, "...")` |
| Échec auth | `return nil, web.Unauthorized("...")` |
| Permission refusée | `return nil, web.Forbidden("...")` |
| Validation (règle métier) | `return nil, web.NewFieldError("champ", "...")` |
| Erreur de domaine (vous la contrôlez) | implémenter l'interface `RequestError` |
| Erreur tierce | enregistrer `ErrorHandler` avec `//helix:handles` |
| Erreur inattendue | la retourner — Helix la mappe en 500 |
