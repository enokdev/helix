# Couche HTTP

Ce guide explique la couche HTTP déclarative de Helix : routing par convention, directives, binding des requêtes, guards, interceptors, mapping des réponses et error handlers. Il se concentre sur les APIs publiques actuellement disponibles.

## Sommaire

- [Modèle mental](#modèle-mental)
- [Routing par convention](#routing-par-convention)
- [Routes custom](#routes-custom)
- [Extracteurs types](#extracteurs-types)
- [Guards](#guards)
- [Interceptors](#interceptors)
- [Mapping des réponses](#mapping-des-réponses)
- [Error handlers](#error-handlers)
- [Tests](#tests)
- [Erreurs fréquentes](#erreurs-fréquentes)

## Modèle mental

Helix sépare le code applicatif du moteur HTTP sous-jacent :

- Les controllers applicatifs utilisent `helix.Controller` et, si besoin, `web.Context`.
- Le serveur est créé par `web.NewServer`.
- Les routes de controller sont enregistrées avec `web.RegisterController`.
- Fiber reste encapsulé dans `web/internal`; un controller utilisateur ne doit pas importer `gofiber/fiber`.

Exemple minimal :

```go
server := web.NewServer()

controller := &UserController{Service: users}
if err := web.RegisterController(server, controller); err != nil {
	return err
}

if err := server.Start(":8080"); err != nil {
	return err
}
```

`web.Context` donne accès aux informations de requête sans exposer Fiber :

```go
func (c *UserController) Show(ctx web.Context) (*User, error) {
	id := ctx.Param("id")
	return c.Service.Find(id)
}
```

Les helpers disponibles incluent `Method`, `Path`, `OriginalURL`, `Param`, `Query`, `Header`, `IP`, `Body`, `Status`, `SetHeader`, `Send`, `JSON` et `Locals`.

## Routing par convention

Un controller routable est un pointeur vers une struct :

- dont le nom finit par `Controller`;
- qui embed `helix.Controller`;
- dont les méthodes publiques suivent les conventions REST ou portent une directive `//helix:route`.

```go
type UserController struct {
	helix.Controller
	Service *UserService
}
```

`web.RegisterController` dérive le préfixe à partir du nom du controller :

- `UserController` devient `/users`;
- `BlogPostController` devient `/blog-posts`;
- les routes sont au pluriel, en kebab-case;
- le paramètre de ressource est `:id`.

Conventions reconnues :

| Méthode controller | Route |
| --- | --- |
| `Index` | `GET /users` |
| `Show` | `GET /users/:id` |
| `Create` | `POST /users` |
| `Update` | `PUT /users/:id` |
| `Delete` | `DELETE /users/:id` |

Exemple complet :

```go
func (c *UserController) Index() ([]User, error) {
	return c.Service.List()
}

func (c *UserController) Show(ctx web.Context) (*User, error) {
	return c.Service.Find(ctx.Param("id"))
}

func (c *UserController) Create(input CreateUserInput) (*User, error) {
	return c.Service.Create(input)
}

func (c *UserController) Update(ctx web.Context, input UpdateUserInput) (*User, error) {
	id := ctx.Param("id")
	return c.Service.Update(id, input)
}

func (c *UserController) Delete(ctx web.Context) error {
	return c.Service.Delete(ctx.Param("id"))
}
```

Une méthode qui ne correspond pas à une convention et qui n'a pas de directive `//helix:route` n'est pas exposée comme route. Les routes statiques custom sont enregistrées avant les routes paramétrées, ce qui évite que `/users/search` soit capturé par `/users/:id`.

## Routes custom

Utilisez `//helix:route` quand la convention ne suffit pas.

```go
//helix:route GET /users/search
func (c *UserController) Search(params SearchUsersQuery) ([]User, error) {
	return c.Service.Search(params)
}
```

Le format est strict :

- deux slashes, sans espace : `//helix:route GET /users/search`;
- exactement une méthode HTTP et un chemin;
- le chemin commence par `/`;
- plusieurs directives `//helix:route` peuvent être placées sur la même méthode.

Exemple avec deux routes :

```go
//helix:route GET /users/search
//helix:route POST /users/search
func (c *UserController) Search(params SearchUsersQuery) ([]User, error) {
	return c.Service.Search(params)
}
```

Ces formes sont invalides et retournent une erreur de directive :

```go
// helix:route GET /users/search
//+helix:route GET /users/search
//helix:route GET
```

Les directives `//helix:guard` et `//helix:interceptor` peuvent être combinées avec une route conventionnelle ou custom.

## Extracteurs types

Une méthode de controller peut recevoir :

- aucun paramètre;
- `web.Context` seul;
- une struct bindée depuis la query string;
- une struct bindée depuis le body JSON;
- `web.Context` en premier paramètre, puis une seule struct bindée.

Exemples valides :

```go
func (c *UserController) Index() ([]User, error)
func (c *UserController) Show(ctx web.Context) (*User, error)
func (c *UserController) Search(params SearchUsersQuery) ([]User, error)
func (c *UserController) Create(ctx web.Context, input CreateUserInput) (*User, error)
```

Une méthode ne peut pas recevoir plusieurs structs bindées. Si `web.Context` est présent, il doit être le premier paramètre.

### Query params

Une struct avec tags `query` est bindée depuis la query string.

```go
type SearchUsersQuery struct {
	Page     int    `query:"page" default:"1" validate:"min=1"`
	PageSize int    `query:"page_size" default:"20" max:"100" validate:"min=1"`
	Email    string `query:"email" validate:"required,email"`
}
```

Comportement utile :

- `query:"page"` indique le nom externe du paramètre.
- `default:"1"` est appliqué si le paramètre est absent ou vide.
- `max:"100"` limite une valeur numérique.
- `validate:"required,email"` est évalué par `go-playground/validator/v10`.
- Les champs non exportés sont ignorés.

Si un paramètre ne peut pas être converti, Helix retourne une erreur structurée `web.RequestError` avec un code comme `INVALID_QUERY_PARAM`. Si la validation échoue, le code est `VALIDATION_FAILED`.

### Body JSON

Une struct avec tags `json` est bindée depuis le body JSON.

```go
type CreateUserInput struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}
```

Le body JSON doit être non vide. Helix refuse les champs inconnus et refuse un body contenant plusieurs valeurs JSON. La validation `validate` est exécutée après le parsing.

Dans cet exemple, `json:"email"` nomme le champ JSON externe et `validate:"required,email"` impose une valeur présente au format email.

Ne mélangez pas `query` et `json` dans la même struct de binding : Helix rejette cette signature comme ambiguë.

## Guards

Un guard décide si une requête peut continuer vers le handler. Une directive de route référence un guard nommé :

```go
//helix:guard authenticated
func (c *UserController) Index() ([]User, error) {
	return c.Service.List()
}
```

La forme exacte `//helix:guard authenticated` référence un guard sans argument. La forme exacte `//helix:guard role:admin` référence une factory de guard avec l'argument `admin`.

Enregistrez le guard sur le serveur avant `web.RegisterController` :

```go
if err := web.RegisterGuard(server, "authenticated", web.GuardFunc(func(ctx web.Context) error {
	if ctx.Header("Authorization") == "" {
		return web.Unauthorized("missing authorization header")
	}
	return nil
})); err != nil {
	return err
}
```

Pour une directive avec argument, utilisez `web.RegisterGuardFactory` :

```go
if err := web.RegisterGuardFactory(server, "role", func(role string) (web.Guard, error) {
	return web.GuardFunc(func(ctx web.Context) error {
		// Note: ctx.Locals("role") doit avoir été peuplé par un middleware de sécurité préalable.
		if ctx.Locals("role") != role {
			return web.Forbidden("insufficient role")
		}
		return nil
	}), nil
}); err != nil {
	return err
}
```

APIs à retenir : `web.RegisterGuard`, `web.RegisterGuardFactory`, `web.ApplyGlobalGuard`, `web.Unauthorized` et `web.Forbidden`.

Puis :

```go
//helix:guard role:admin
func (c *UserController) Delete(ctx web.Context) error {
	return c.Service.Delete(ctx.Param("id"))
}
```

`web.ApplyGlobalGuard` ajoute un guard exécuté avant toutes les routes du serveur. Les guards globaux passent avant les guards déclarés sur une route. Les guards de route s'exécutent dans l'ordre des directives.

**Comportement en cas d'échec** : Si un guard retourne une erreur, l'exécution de la chaîne s'arrête immédiatement. Les guards suivants, les interceptors et le handler ne sont pas appelés.

Utilisez `web.Unauthorized` pour produire une réponse `401` structurée et `web.Forbidden` pour une réponse `403` structurée.

Une directive guard doit avoir un seul argument. Les noms de directive sont en minuscules, peuvent contenir des chiffres après le premier caractère, et peuvent utiliser des tirets internes. Les formes invalides, les guards non enregistrés et les directives sur méthodes non routables retournent `web.ErrInvalidDirective`.

## Interceptors

Un interceptor entoure l'appel au handler. Il peut enrichir la requête, mesurer une durée, ajouter des headers ou court-circuiter la réponse.

```go
if err := web.RegisterInterceptor(server, "trace", web.InterceptorFunc(func(ctx web.Context, next web.HandlerFunc) error {
	ctx.SetHeader("X-Trace", "enabled")
	return next(ctx)
})); err != nil {
	return err
}
```

APIs à retenir : `web.RegisterInterceptor` et `web.RegisterInterceptorFactory`.

Puis :

```go
//helix:interceptor trace
func (c *UserController) Index() ([]User, error) {
	return c.Service.List()
}
```

Pour une directive avec argument, utilisez `web.RegisterInterceptorFactory`.

Helix fournit déjà un interceptor de cache sous le nom `cache` :

```go
//helix:interceptor cache:5m
func (c *UserController) Index() ([]User, error) {
	return c.Service.List()
}
```

La forme exacte `//helix:interceptor cache:5m` active l'interceptor intégré avec une durée de cache de cinq minutes.

N'enregistrez pas un autre interceptor sous le nom `cache` : le serveur le réserve déjà. Les interceptors de route s'empilent dans l'ordre des directives autour du handler. Les guards s'exécutent avant les interceptors.

## Mapping des réponses

Les handlers peuvent retourner un payload, une erreur, les deux, ou rien selon la signature supportée par `web.RegisterController`.

Succès :

- une route `POST` écrit `201 Created`;
- les autres méthodes écrivent `200 OK`;
- le body JSON est direct, sans wrapper `data`.

**Cas (nil, nil)** : Si un handler retourne à la fois un payload nul et une erreur nulle, Helix répond avec un `200 OK` et un corps de réponse vide.

Exemple de réponse :

```json
{
  "id": "42",
  "email": "alice@example.com"
}
```

Erreur :

```json
{
  "error": {
    "type": "ValidationError",
    "message": "email is required",
    "field": "email",
    "code": "VALIDATION_FAILED"
  }
}
```

`web.RequestError` couvre les erreurs de binding et validation. Les erreurs de guard produites par `web.Unauthorized` et `web.Forbidden` utilisent la même enveloppe. Une erreur générique non structurée retourne un `InternalServerError` avec le message public `internal server error`.

Pour une erreur applicative personnalisée, utilisez soit un type qui expose les informations HTTP structurées attendues par Helix, soit un error handler centralisé.

## Error handlers

Un error handler centralisé transforme un type d'erreur applicatif en réponse HTTP.

```go
type APIErrorHandler struct {
	helix.ErrorHandler
}

//helix:handles ValidationError
func (h *APIErrorHandler) Validation(ctx web.Context, err error) error {
	ctx.Status(400)
	// Note: Veillez à ne pas exposer de détails techniques non sécurisés via err.Error()
	// Préférez des messages d'erreur contrôlés.
	return ctx.JSON(web.ErrorResponse{
		Error: web.ErrorDetail{
			Type:    "ValidationError",
			Message: "Données de requête invalides",
			Code:    "VALIDATION_FAILED",
		},
	})
}
```

Enregistrez le handler avant de servir les requêtes :

```go
if err := web.RegisterErrorHandler(server, &APIErrorHandler{}); err != nil {
	return err
}
```

L'API publique à utiliser est `web.RegisterErrorHandler`.

Contraintes :

- la struct doit être un pointeur non nil;
- son nom doit finir par `ErrorHandler`;
- elle doit embed `helix.ErrorHandler`;
- chaque méthode gérée porte une seule directive `//helix:handles ValidationError`;
- le type dans la directive doit être un identifiant Go valide, par exemple `ValidationError` ou `NotFoundError`.

`//helix:handles` n'est pas une directive de route. Elle est traitée par le registre d'error handlers.

## Tests

`web.HTTPServer` expose `ServeHTTP`, pratique pour tester sans ouvrir de port réseau.

```go
server := web.NewServer()
if err := web.RegisterController(server, &UserController{Service: users}); err != nil {
	t.Fatal(err)
}

req := httptest.NewRequest(http.MethodGet, "/users", nil)
res, err := server.ServeHTTP(req)
if err != nil {
	t.Fatal(err)
}
defer res.Body.Close()

if res.StatusCode != http.StatusOK {
	t.Fatalf("status = %d", res.StatusCode)
}
```

Bonnes pratiques :

- gardez les tests de routes table-driven quand plusieurs cas partagent le même setup;
- testez les chemins d'erreur de binding, validation, guards et directives invalides;
- vérifiez les codes HTTP et les champs `error.type`, `error.code`, `error.field`;
- n'importez pas Fiber dans les tests de controller utilisateur.

## Erreurs fréquentes

- Importer `gofiber/fiber` dans un controller : utilisez `web.Context`.
- Oublier `helix.Controller` ou le suffixe `Controller` : `web.RegisterController` rejettera la struct.
- Écrire `// helix:route` ou `//+helix:route` : la directive doit commencer par `//helix:route`.
- Placer `//helix:guard` ou `//helix:interceptor` sur une méthode non routable : ajoutez une convention reconnue ou `//helix:route`.
- Référencer un guard ou interceptor non enregistré avant `web.RegisterController`.
- Mélanger `query` et `json` dans la même struct de binding.
- Promettre un `404` automatique pour `nil, nil` : retournez une erreur structurée ou utilisez un error handler.
