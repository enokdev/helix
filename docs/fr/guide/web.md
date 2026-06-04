# Web & Routage

La couche HTTP d'Helix est construite sur [Fiber](https://gofiber.io/) et fournit le routage par convention, le binding automatique des requêtes, le mapping des réponses, les guards, les interceptors et la gestion centralisée des erreurs.

## Contrôleurs

Un contrôleur est tout struct qui intègre `helix.Controller`. Helix l'auto-enregistre dans le routeur quand il est passé à `helix.Run` ou `web.RegisterController`.

```go
type ProductController struct {
    helix.Controller
    Svc *ProductService `inject:"true"`
}
```

La route de base est dérivée du nom du struct en supprimant le suffixe `Controller` et en mettant en minuscules :
`ProductController` → `/products`

Surchargez avec un tag de struct :

```go
type ProductController struct {
    helix.Controller `helix:"route:/api/v1/products"`
    Svc *ProductService `inject:"true"`
}
```

## Routage par convention

Nommez les méthodes selon les conventions REST et les routes sont enregistrées automatiquement :

| Méthode | Route HTTP | Statut |
|---------|-----------|--------|
| `Index()` | `GET /products` | 200 |
| `Show()` | `GET /products/:id` | 200 |
| `Create()` | `POST /products` | 201 |
| `Update()` | `PUT /products/:id` | 200 |
| `Delete()` | `DELETE /products/:id` | 204 |

```go
// GET /products
func (c *ProductController) Index() []Product {
    return c.Svc.List()
}

// GET /products/:id
func (c *ProductController) Show(ctx web.Context) (Product, error) {
    id, _ := strconv.Atoi(ctx.Param("id"))
    return c.Svc.Get(id)
}

// POST /products  →  201 Created
func (c *ProductController) Create(ctx web.Context, input CreateProductInput) (Product, error) {
    return c.Svc.Create(input)
}

// PUT /products/:id
func (c *ProductController) Update(ctx web.Context, input UpdateProductInput) (Product, error) {
    id, _ := strconv.Atoi(ctx.Param("id"))
    return c.Svc.Update(id, input)
}

// DELETE /products/:id
func (c *ProductController) Delete(ctx web.Context) error {
    id, _ := strconv.Atoi(ctx.Param("id"))
    return c.Svc.Delete(id)
}
```

## Routes personnalisées

Utilisez les directives `//helix:route METHOD /chemin` pour les routes hors convention :

```go
//helix:route POST /auth/login
func (c *AuthController) Login(ctx web.Context, req LoginRequest) (LoginResponse, error) {
    return c.AuthSvc.Authenticate(req.Username, req.Password)
}

//helix:route GET /products/featured
func (c *ProductController) Featured() []Product {
    return c.Svc.GetFeatured()
}
```

## Le contexte

`web.Context` donne accès à la requête et à la réponse :

```go
// Requête
ctx.Method()         // "GET", "POST", ...
ctx.Path()           // "/products/42"
ctx.OriginalURL()    // "/products/42?page=1"
ctx.Param("id")      // paramètre de route
ctx.Query("page")    // paramètre de query string
ctx.Header("X-Trace-ID")
ctx.IP()
ctx.Body()           // []byte bruts

// Réponse
ctx.Status(201)
ctx.SetHeader("X-Request-ID", id)
ctx.AppendHeader("Vary", "Accept-Encoding")
ctx.Send([]byte("ok"))
ctx.JSON(map[string]any{"status": "ok"})

// Locals (stockage local à la requête, défini par guards/interceptors)
ctx.Locals("user")              // lecture
ctx.Locals("user", userClaims) // écriture
```

## Types de retour des handlers

Helix inspecte les valeurs de retour du handler pour décider de la réponse HTTP :

| Signature de retour | Corps | Statut par défaut |
|--------------------|-------|------------------|
| `func() error` | vide | 204 (ou statut d'erreur) |
| `func() (T, error)` | T encodé en JSON | 200 |
| Méthode `Create()` | T encodé en JSON | 201 |
| Méthode `Delete()` | vide | 204 |
| `func(ctx web.Context)` | défini via `ctx.JSON()`/`ctx.Send()` | ce que vous définissez |
| retourne `(nil, nil)` | vide | 204 |
| `ctx.Status(n)` explicite | surcharge le défaut | n |

## Binding de requête

### Paramètres de query

```go
type ListProductsQuery struct {
    Page     int    `query:"page"`
    PageSize int    `query:"pageSize"`
    Category string `query:"category"`
}

//helix:route GET /products/search
func (c *ProductController) Search(ctx web.Context, q ListProductsQuery) ([]Product, error) {
    // q est automatiquement rempli depuis ?page=1&pageSize=20&category=electronics
    return c.Svc.Search(q.Category, q.Page, q.PageSize)
}
```

### Corps JSON

Les paramètres de méthode après `web.Context` avec un type struct sont bindés depuis le corps de la requête :

```go
type CreateProductInput struct {
    Name        string  `json:"name"        validate:"required,min=2,max=100"`
    Price       float64 `json:"price"       validate:"required,gt=0"`
    CategoryID  int     `json:"categoryId"  validate:"required"`
    Description string  `json:"description"`
}

func (c *ProductController) Create(ctx web.Context, input CreateProductInput) (Product, error) {
    // input est bindé et validé automatiquement
    // retourne 400 avec les erreurs de champs si la validation échoue
    return c.Svc.Create(input)
}
```

La validation utilise [go-playground/validator](https://github.com/go-playground/validator). Tous les tags de validation standards sont supportés.

### Réponse d'erreur de validation

```json
{
  "errors": [
    { "type": "ValidationError", "field": "price",      "message": "doit être supérieur à 0" },
    { "type": "ValidationError", "field": "categoryId", "message": "requis" }
  ]
}
```

### Binding imbriqué

Les paramètres de query et les corps JSON supportent les structs imbriqués :

```go
type SearchFilter struct {
    MinPrice float64 `query:"minPrice"`
    MaxPrice float64 `query:"maxPrice"`
    InStock  bool    `query:"inStock"`
}

type SearchQuery struct {
    Query    string       `query:"q"`
    Page     int          `query:"page"`
    Filter   SearchFilter `query:"filter"` // bindé depuis filter.minPrice, filter.maxPrice, etc.
}

//helix:route GET /products/search
func (c *ProductController) Search(ctx web.Context, q SearchQuery) ([]Product, error) {
    // ?q=clavier&page=1&filter.minPrice=50&filter.maxPrice=200&filter.inStock=true
    return c.Svc.Search(q)
}
```

## Guards

Les guards protègent les routes des accès non autorisés. Ils s'exécutent avant le handler.

### Guard auth intégré

```go
//helix:guard auth
func (c *APIController) Profile(ctx web.Context) (UserProfile, error) {
    // accessible uniquement avec un token JWT Bearer valide
    claims, _ := ctx.Locals("claims").(map[string]any)
    return c.UserSvc.GetProfile(claims["sub"].(string))
}
```

### Guard de rôle

```go
//helix:guard role:admin
func (c *AdminController) Users(_ web.Context) ([]User, error) {
    return c.UserSvc.ListAll()
}
```

### Guards multiples

Empilez plusieurs directives `//helix:guard` sur la même méthode — tous doivent passer :

```go
//helix:guard auth
//helix:guard rate-limit
//helix:guard role:admin
func (c *AdminController) Export(_ web.Context) (ExportData, error) {
    // doit : avoir un JWT valide ET ne pas être rate-limité ET avoir le rôle admin
    return c.Svc.Export()
}
```

Les guards s'exécutent dans l'ordre de déclaration. Le premier qui retourne une erreur court-circuite la chaîne.

### Guard custom

```go
type RateLimitGuard struct{}

func (g *RateLimitGuard) CanActivate(ctx web.Context) error {
    if isRateLimited(ctx.IP()) {
        return web.NewRequestError(http.StatusTooManyRequests, "limite de taux dépassée")
    }
    return nil
}

// Enregistrement :
web.RegisterGuard(server, "rate-limit", &RateLimitGuard{})

// Utilisation :
//helix:guard rate-limit
func (c *APIController) Search(...) { ... }
```

### Guard global

Appliquer un guard à toutes les routes :

```go
web.ApplyGlobalGuard(server, jwtGuard)
```

## Interceptors

Les interceptors enveloppent les handlers — utiles pour le cache, les logs, le tracing ou la transformation de requêtes.

### Ordre d'exécution

Quand plusieurs interceptors sont empilés, ils s'enveloppent comme des couches d'oignon — le code `avant` s'exécute dans l'ordre de déclaration, le code `après` s'exécute en sens inverse :

```go
//helix:interceptor log
//helix:interceptor cache
func (c *ProductController) Index() []Product { ... }
```

Ordre d'exécution :
```
log.avant → cache.avant → handler → cache.après → log.après
```

```go
type AuditInterceptor struct{}

func (i *AuditInterceptor) Intercept(ctx web.Context, next web.HandlerFunc) error {
    start := time.Now()
    err := next(ctx)  // ← exécute l'interceptor cache + le handler
    slog.Info("requête", "méthode", ctx.Method(), "chemin", ctx.Path(),
        "durée", time.Since(start), "erreur", err)
    return err
}

// Enregistrement :
web.RegisterInterceptor(server, "audit", &AuditInterceptor{})

// Utilisation :
//helix:interceptor audit
func (c *ProductController) Create(...) { ... }
```

## Interceptor de cache

Helix inclut un interceptor de cache integre pour les reponses GET :

```go
//helix:interceptor cache
func (c *ProductController) Index() []Product { ... }

// TTL personnalise par route :
//helix:interceptor cache:5m
func (c *ProductController) Show(ctx web.Context) (Product, error) { ... }

// TTL, limite d'entrees et strategie d'eviction :
//helix:interceptor cache:30s:max=500:lru
func (c *ProductController) Search(ctx web.Context) ([]Product, error) { ... }
```

Options de directive :

- `cache:<duration>` definit le TTL avec la syntaxe de duree Go, par exemple `cache:30s` ou `cache:5m`.
- `cache:<duration>:max=<entries>` limite le nombre d'entrees stockees pour cet interceptor.
- `cache:<duration>:lru` utilise l'eviction least-recently-used ; `cache:<duration>:fifo` evince les entrees les plus anciennes.

Seules les reponses GET reussies et ecrites en JSON sont mises en cache. Les requetes non-GET, les erreurs de handler, les reponses non-2xx, les reponses non-JSON et les reponses plus grandes que la limite de corps du cache traversent l'interceptor sans etre stockees.

Les requetes concurrentes a froid pour la meme URL sont coalescees : la premiere requete execute le handler, puis les requetes en attente rejouent le meme resultat JSON quand il est disponible. Les cles de cache sont calculees avec la methode HTTP et l'URL originale, query string incluse. Les entrees expirees sont supprimees paresseusement a l'acces et par un sweep periodique en arriere-plan.

## Gestion des erreurs

### Retourner des erreurs depuis les handlers

Helix mappe les erreurs retournées vers des codes HTTP automatiquement :

```go
func (c *ProductController) Show(ctx web.Context) (Product, error) {
    id, _ := strconv.Atoi(ctx.Param("id"))
    p, ok := c.Svc.Get(id)
    if !ok {
        return Product{}, web.NewRequestError(http.StatusNotFound, "produit introuvable")
    }
    return p, nil
}
```

```json
// Réponse 404
{
  "error": {
    "type": "NotFound",
    "message": "produit introuvable"
  }
}
```

### Erreurs utilitaires

```go
web.Unauthorized("identifiants invalides")      // 401
web.Forbidden("permissions insuffisantes")      // 403
web.NewRequestError(404, "introuvable")         // tout 4xx
```

Pour la gestion complète des erreurs, voir [Gestion des erreurs](/fr/guide/error-handling).

## Utilisation manuelle du serveur

Vous pouvez aussi utiliser le serveur HTTP directement sans `helix.Run` :

```go
server := web.NewServer(
    web.WithRouteObserver(metricsObserver),
    web.WithTracerProvider(tp),
)

web.RegisterController(server, &ProductController{})
web.ApplyGlobalGuard(server, jwtGuard)

server.Start(":8080")
```
