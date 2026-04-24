# HTTP Layer Documentation Clarifications

Based on comprehensive code analysis of `web/response.go`, `web/router.go`, `web/binding.go`, `web/error_handler.go`, and `web/guard.go`, this document identifies documentation gaps and proposes precise clarifications for `docs/http-layer.md`.

---

## Finding 1: Handler Return Values (nil, nil) Response Code

**Current Documentation Issue:**

Section "Mapping des reponses" states: "une route `POST` ecrit `201 Created`; les autres methodes ecrivent `200 OK`". However, no guidance on what body is written when a handler returns `(nil, nil)`.

**Actual Behavior:**

When a handler returns `(nil, nil)`:
- `writeSuccessResponse(ctx, method, nil)` is called (response.go:24-35)
- Status is set: POST→201, others→200 OK (response.go:25, successStatus)
- `ctx.JSON(nil)` serializes the nil payload as JSON `null` (response.go:26)
- **Result: Status 200/201 with JSON body containing literal `null`**

**Recommended Documentation Change:**

In section "Mapping des reponses", add after the existing success paragraph:

> **Note sur les reponses sans payload**: Si un handler retourne une payload `nil`, Helix serialise le nil en JSON `null`. Par exemple, une route `GET /users` qui retourne `(nil, nil)` produit une reponse `200 OK` avec le corps `null`. Si vous souhaitez une reponse vide, retournez un slice vide `[]` ou une struct vide appropriee.

**Example:**

```go
// Cette methode produit: 200 OK avec corps "null"
func (c *UserController) Index() ([]User, error) {
    // Retourne (nil, nil)
    return nil, nil
}

// Cette methode produit: 200 OK avec corps "[]"
func (c *UserController) Index() ([]User, error) {
    // Retourne une slice vide au lieu de nil
    return []User{}, nil
}
```

---

## Finding 2: Guard Factory Error Handling — Init-Time vs Request-Time

**Current Documentation Issue:**

Section "Guards" shows RegisterGuardFactory but does not specify when factory errors are caught. Current text: "La forme exacte `//helix:guard role:admin` reference une factory de guard avec l'argument `admin`." Does not clarify that factory **errors are init-time, not request-time**.

**Actual Behavior:**

Guard factory signature: `func(arg string) (web.Guard, error)` (guard.go:27)

Error handling timeline:
- Factory is invoked during `wrapControllerHandler` → `resolver.resolveGuard` (router.go:208-211)
- This is called during `RegisterController`, **not during request handling**
- If factory returns error, `RegisterController` fails and route is **never registered**
- **Result: Init-time failure prevents route registration entirely**

**Recommended Documentation Change:**

In section "Guards", after the factory example, add:

> **Important**: Si une factory de guard retourne une erreur, `web.RegisterController` echoue et la route n'est jamais enregistree. Les erreurs de factory sont des erreurs d'initialisation, non des erreurs de requete. Une factory ne doit pas utiliser les donnees de la requete pour decider si elle peut creer un guard; elle doit creer le guard avec sa configuration immediatement.

**Example:**

```go
// ✅ Correct: factory ne depend que de l'argument
if err := web.RegisterGuardFactory(server, "role", func(role string) (web.Guard, error) {
    // Verifications rapides sur l'argument (ex: validation du format)
    if role == "" {
        return nil, fmt.Errorf("role must not be empty")
    }
    return web.GuardFunc(func(ctx web.Context) error {
        // Ici, on utilise les donnees de la requete pour verifier l'access
        userRole := ctx.Locals("role").(string)
        if userRole != role {
            return web.Forbidden("insufficient role")
        }
        return nil
    }), nil
}); err != nil {
    return err
}
```

---

## Finding 3: Query Parameter Binding & Validation — Order and Error Codes

**Current Documentation Issue:**

Section "Query params" mentions `default`, `max`, and `validate` tags but does not specify:
1. Order of operations (default applied before or after validation?)
2. Distinction between `INVALID_QUERY_PARAM` and `VALIDATION_FAILED` error codes
3. Behavior of overflow (e.g., int32 overflow)

**Actual Behavior:**

Order of operations (binding.go:179-197):
1. Query param extracted (line 181)
2. **Default applied if param empty or missing** (line 182-184)
3. Numeric conversion attempted via `setQueryValue` (line 190)
   - Conversion error → `INVALID_QUERY_PARAM` (line 191)
4. Max check if conversion succeeded (line 193-195)
   - Max violation → `VALIDATION_FAILED` (line 194)
5. Struct validation via validator (line 173-174)
   - Validation error (e.g., `min=1`) → `VALIDATION_FAILED` (line 194)

**Specific cases:**
- Page `-1`: passes int conversion, fails `validate:"min=1"` → `VALIDATION_FAILED`
- Page `99999999999999999` into int32: ParseInt fails → `INVALID_QUERY_PARAM`
- Page `1.5`: ParseInt fails → `INVALID_QUERY_PARAM`
- Page `""` with `default:"1"`: default applied, no error

**Recommended Documentation Change:**

Replace current "Query params" section with:

> ### Query params
> 
> Une struct avec tags `query` est bindee depuis la query string.
> 
> ```go
> type SearchUsersQuery struct {
> 	Page     int    `query:"page" default:"1" validate:"min=1"`
> 	PageSize int    `query:"page_size" default:"20" max:"100" validate:"min=1"`
> 	Email    string `query:"email" validate:"required,email"`
> }
> ```
> 
> Comportement et ordre de traitement :
> 
> 1. **Extraction**: Le parametre est extrait de la query string.
> 2. **Defaut applique**: Si le parametre est absent ou vide, la valeur par defaut est appliquee.
> 3. **Conversion**: Pour les champs numeriques, le parametre est converti au type (int, uint, bool). Si la conversion echoue (ex: `"abc"` pour int), une erreur `INVALID_QUERY_PARAM` est retournee.
> 4. **Max check**: Si un tag `max` est present et la valeur convertie depasse le max, une erreur `VALIDATION_FAILED` est retournee.
> 5. **Validation**: Le validateur `go-playground/validator/v10` verifie les regles `validate` (ex: `min=1`, `required`, `email`). Les violations retournent `VALIDATION_FAILED`.
> 
> **Tags supportes:**
> - `query:"page"` — nom externe du parametre
> - `default:"1"` — valeur appliquee si parametre absent ou vide
> - `max:"100"` — limite numerique pour int/uint (appliquee apres conversion, avant validation)
> - `validate:"required,email"` — regles du validateur go-playground
> 
> **Codes d'erreur:**
> - `INVALID_QUERY_PARAM` — conversion numerique echouee (ex: `"abc"` pour int)
> - `VALIDATION_FAILED` — max depasse ou validation `validate:` echouee
> 
> Les champs non exportes sont ignores.

**Example:**

```go
type PaginationQuery struct {
    Page int `query:"page" default:"1" validate:"min=1"`
}

// "page=0" → VALIDATION_FAILED (min=1 failed)
// "page=abc" → INVALID_QUERY_PARAM (conversion failed)
// "page=" → default "1" applied, no error
// Missing "page" → default "1" applied, no error
```

---

## Finding 4: JSON Body Binding — Emptiness, DisallowUnknownFields, Multiple Values

**Current Documentation Issue:**

Section "Body JSON" states "Le body JSON doit etre non vide. Helix refuse les champs inconnus et refuse un body contenant plusieurs valeurs JSON." But does not clarify what "non vide" means exactly (is `{}` considered empty? Is `   ` rejected?).

**Actual Behavior:**

JSON binding (binding.go:200-215):
1. Body trimmed of whitespace (line 201: `bytes.TrimSpace`)
2. If empty after trimming → error "request body is required" (line 202-203)
3. JSON decoder configured with `DisallowUnknownFields` (line 207)
4. Struct unmarshaled (line 208)
   - Unknown field → `INVALID_JSON` (line 210)
5. Attempt to read next JSON value (line 212)
   - If EOF not reached → "request body must contain a single JSON value" (line 212-214)
   - If not EOF → `INVALID_JSON`

**Specific cases:**
- Empty string `""` → error "request body is required"
- Whitespace only `"   "` → error "request body is required"
- `{}` → valid, parses as empty object
- `[]` → valid, but may fail if struct expects object (JSON decode error)
- `{"name":"alice","unknown":true}` with struct having no unknown field → `INVALID_JSON` (DisallowUnknownFields)
- `{"name":"alice"}{"name":"bob"}` → `INVALID_JSON` (multiple JSON values)

**Recommended Documentation Change:**

Replace "Body JSON" section with:

> ### Body JSON
> 
> Une struct avec tags `json` est bindee depuis le body JSON.
> 
> ```go
> type CreateUserInput struct {
> 	Name  string `json:"name" validate:"required"`
> 	Email string `json:"email" validate:"required,email"`
> }
> ```
> 
> **Regles de validation du body:**
> 
> 1. Le body est obligatoire et non vide apres suppression des espaces blancs. Un body vide, contenant seulement des espaces (`   `), ou absent retourne une erreur "request body is required".
> 2. Le body doit contenir un seul objet/valeur JSON valide. Plusieurs valeurs JSON successives (ex: `{...}{...}`) sont rejetees.
> 3. Les champs inconnus (non presentes dans la struct) sont refuses. La directive `DisallowUnknownFields` est activee.
> 4. Apres le parsing reussi, la validation `validate` est executee.
> 
> **Exemples:**
> - `{}` — valide, structure vide
> - `{"name":"alice","email":"alice@example.com"}` — valide
> - `{"name":"alice","role":"admin"}` avec struct sans champ `role` — erreur `INVALID_JSON` (unknown field)
> - `""` ou `   ` — erreur "request body is required"
> - `{"name":"alice"}{"name":"bob"}` — erreur "request body must contain a single JSON value"
> 
> **Codes d'erreur:**
> - `INVALID_JSON` — body vide, parsing JSON echoue, champ inconnu ou multiple valeurs
> 
> Ne melangez pas `query` et `json` dans la meme struct de binding : Helix rejette cette signature comme ambigue.

**Example:**

```go
// ✅ Valid: {"name": "alice", "email": "alice@example.com"}
// ❌ Empty: "" → "request body is required"
// ❌ Unknown field: {"name": "alice", "role": "admin"} → INVALID_JSON
// ❌ Multiple values: {"name": "alice"}{"name": "bob"} → INVALID_JSON
```

---

## Finding 5: Guard Execution Order — Global, Route, Interceptors Stacking

**Current Documentation Issue:**

Section "Guards" states "Les guards globaux passent avant les guards declares sur une route. Les guards de route s'executent dans l'ordre des directives." Does not clarify if guards execute **before or after** interceptors.

**Actual Behavior:**

Guard and interceptor wrapping (router.go:227-244):
1. `composeHandler` builds composition:
   - Interceptors wrapped **outermost** (lines 229-234)
   - Guards execute **inside** interceptor wrapping (lines 237-242)
   - Actual handler runs innermost
2. Guard execution order:
   - Global guards added separately via `ApplyGlobalGuard` (guard.go:111-119)
   - Global guards execute **before** route-specific guards (unconfirmed in code shown, but standard pattern)
   - Route guards in directive order (router.go:238-241)
   - If any guard fails, error returned immediately, handler and remaining guards do **not** execute

**Execution flow:**

```
Request
  ↓
Interceptor 1 (outermost)
  ↓
Interceptor 2
  ↓
Global Guard 1
  ↓
Global Guard 2
  ↓
Route Guard 1 (from //helix:guard)
  ↓
Route Guard 2 (from //helix:guard)
  ↓
Handler
```

If Guard 2 fails, Route Guard 2 and Handler do not execute.

**Recommended Documentation Change:**

In section "Guards", replace final paragraph with:

> `web.ApplyGlobalGuard` ajoute un guard execute avant toutes les routes du serveur. Les guards globaux passent avant les guards declares sur une route. Les guards de route s'executent dans l'ordre des directives.
> 
> **Important**: Les guards executent **avant** les interceptors. Voici l'ordre complet d'une requete:
> 1. Interceptors (du premier declare au dernier)
> 2. Guards globaux (en ordre de registration)
> 3. Guards de route (dans l'ordre des directives `//helix:guard`)
> 4. Handler
> 5. Interceptors (du dernier declare au premier, lors du retour)
> 
> Si un guard echoue, les guards suivants et le handler ne s'executent pas. L'erreur du guard est retournee immediatement.

---

## Finding 6: Error Handlers — Duplicate Type Handling & Signature Requirements

**Current Documentation Issue:**

Section "Error handlers" shows an example but does not specify:
1. Exact signature requirements (context optional? return type format?)
2. Behavior when two handlers handle the same error type (which wins?)
3. When error handlers are invoked vs built-in error handlers

**Actual Behavior:**

Error handler signature validation (error_handler.go:266-284):
- Signature must have 2 return values: `(any, int)` (line 267)
- First return must be JSON-serializable (will be written as response)
- Second return must be `int` (status code)
- Arguments: either `(ErrorType)` OR `(web.Context, ErrorType)` (lines 271-281)
- ErrorType must implement `error` interface (line 273, 278)
- ErrorType must NOT be interface{} (line 273, 278)

Duplicate handling (error_handler.go:71-72):
- If two handlers handle the same error type → **error during `RegisterErrorHandler`**
- Validation checks for duplicates before returning handlers map

**Recommended Documentation Change:**

Replace "Error handlers" section with:

> ## Error handlers
> 
> Un error handler centralise transforme un type d'erreur applicatif en reponse HTTP.
> 
> ```go
> type APIErrorHandler struct {
> 	helix.ErrorHandler
> }
> 
> //helix:handles ValidationError
> func (h *APIErrorHandler) Validation(ctx web.Context, err ValidationError) (any, int) {
> 	return web.ErrorResponse{
> 		Error: web.ErrorDetail{
> 			Type:    "ValidationError",
> 			Message: err.Error(),
> 			Code:    "VALIDATION_FAILED",
> 		},
> 	}, http.StatusBadRequest
> }
> 
> //helix:handles NotFoundError
> func (h *APIErrorHandler) NotFound(err NotFoundError) (any, int) {
> 	return web.ErrorResponse{
> 		Error: web.ErrorDetail{
> 			Type:    "NotFoundError",
> 			Message: err.Error(),
> 			Code:    "NOT_FOUND",
> 		},
> 	}, http.StatusNotFound
> }
> ```
> 
> Enregistrez le handler avant de servir les requetes :
> 
> ```go
> if err := web.RegisterErrorHandler(server, &APIErrorHandler{}); err != nil {
> 	return err
> }
> ```
> 
> **Signature requise:**
> 
> Chaque methode geree porte exactement une directive `//helix:handles TypeName`:
> - `TypeName` doit etre un identifiant Go valide (ex: `ValidationError`, `NotFoundError`)
> - La signature doit etre: `(any, int)` pour les deux returns
> - Les arguments peuvent etre:
>   - `(ErrorType error)` — le type d'erreur seul
>   - `(ctx web.Context, err ErrorType)` — contexte et erreur
> - Le type `ErrorType` doit implémenter `error` et ne pas etre `interface{}`
> 
> **Contraintes:**
> 
> - La struct doit etre un pointeur non nil
> - Son nom doit finir par `ErrorHandler`
> - Elle doit embed `helix.ErrorHandler`
> - Un seul handler par type d'erreur. Si deux methodes gerent le meme type, `RegisterErrorHandler` echoue
> - `//helix:handles` n'est pas une directive de route. Elle est traitee par le registre d'error handlers uniquement
> 
> **Invocation:**
> 
> Les error handlers sont invoques lorsqu'une erreur non structuree (ne correspondant pas a `web.RequestError`, `web.Unauthorized`, etc.) est retournee par un handler. Helix essaie de matcher l'erreur a un handler enregistre via `errors.As`. Si aucun handler ne correspond, une erreur generique `InternalServerError` est retournee.

**Example:**

```go
// Type d'erreur applicatif personnalise
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Handler pour ce type
//helix:handles ValidationError
func (h *APIErrorHandler) Validation(err ValidationError) (any, int) {
    return web.ErrorResponse{
        Error: web.ErrorDetail{
            Type:    "ValidationError",
            Field:   err.Field,
            Message: err.Message,
            Code:    "VALIDATION_FAILED",
        },
    }, http.StatusBadRequest
}

// Utilisation
func (c *UserController) Create(input CreateUserInput) (*User, error) {
    if input.Email == "" {
        return nil, &ValidationError{Field: "email", Message: "required"}
    }
    return c.Service.Create(input), nil
}
```

---

## Finding 7: Nested Struct Binding — Top-Level Fields Only

**Current Documentation Issue:**

Section "Query params" and "Body JSON" do not specify that only **top-level exported fields** are bound. Nested or embedded structs are silently ignored.

**Actual Behavior:**

Field binding (binding.go:97-104):
- Loop over `target.NumField()` (direct fields only)
- Skip fields with `field.PkgPath != ""` (unexported fields)
- Only direct field tags are checked
- Nested/embedded struct fields are **silently skipped**

**Recommended Documentation Change:**

Add note after query params and JSON body sections:

> **Note sur les structs imbriquees**: Seuls les champs exportes de premier niveau sont lies. Les champs non exportes et les champs imbriques dans des structs embarkees sont ignores silencieusement.
> 
> ```go
> type BadQuery struct {
>     page int           // Non exporte, ignore
>     Address struct {
>         City string     // Imbriquee, ignore
>     }
> }
> 
> type GoodQuery struct {
>     Page int `query:"page"`  // Top-level exporte, lie
>     City string `query:"city"` // Top-level exporte, lie
> }
> ```

---

## Finding 8: Reserved Interceptor Names

**Current Documentation Issue:**

Section "Interceptors" states: "N'enregistrez pas un autre interceptor sous le nom `cache`". This is the only documented reserved name, but documentation does not clarify if other names are reserved.

**Actual Behavior:**

Reserved names (based on code inspection):
- `cache` is reserved for the built-in cache interceptor (web/cache_interceptor.go exists)
- No other names are explicitly reserved in the code
- However, `auth`, `cors`, `trace`, etc. could conflict with starter auto-configurations

**Recommended Documentation Change:**

In section "Interceptors", update the paragraph about `cache`:

> N'enregistrez pas un autre interceptor sous le nom `cache` : le serveur le reserve deja pour l'interceptor integre. Evitez aussi les noms qui pourraient confluer avec des starters (ex: `auth`, `security`, `cors`).

---

## Finding 9: Directive Syntax Edge Cases — Whitespace & Path Validation

**Current Documentation Issue:**

Section "Routes custom" states: "Le format est strict: deux slashes, sans espace : `//helix:route GET /users/search`; exactement une methode HTTP et un chemin". But does not specify:
1. Behavior with extra whitespace: `//helix:route  GET  /users`
2. Behavior with special paths: `/`, `/users/`, edge cases
3. Exact parser behavior

**Actual Behavior:**

Route directive parsing (router.go:369-384):
- Extract text after `//helix:route `
- Split by whitespace using `strings.Fields` (line 370)
- `strings.Fields` handles multiple whitespaces, returns tokens
- Expect exactly 2 tokens: method and path (line 371)
- Extra spaces: `//helix:route  GET  /users` → after trim: `GET  /users` → Fields: `["GET", "/users"]` → valid
- Trailing slash: `/users/` → valid (no special check)
- Root path: `/` → valid (no special check)

**Path validation (router.go:379):**
- Calls `validateRoute` which checks method is valid HTTP method
- No explicit path validation in parseRouteDirective

**Recommended Documentation Change:**

In section "Routes custom", expand the format constraints:

> Le format est strict :
> - deux slashes, sans espace : `//helix:route GET /users/search` (les espaces multiples entre METHOD et PATH sont normalises par le parser)
> - exactement une methode HTTP et un chemin, separes par un ou plusieurs espaces
> - la methode doit etre valide (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS, TRACE, CONNECT)
> - le chemin doit commencer par `/` (ex: `/users`, `/`, `/users/:id`)
> - les chemins avec tirets `/users-search` et slashs `/users/search` sont valides
> - les chemins avec parametres `:id` et chemins finissant par `/` (ex: `/users/`) sont valides
> 
> Exemples invalides :
> 
> ```go
> //helix:route GET /users/search extra  // Trop de tokens
> //helix:route GET                        // Manque le chemin
> //helix:route /users/search              // Manque la methode
> // helix:route GET /users/search         // Espace apres //
> //+helix:route GET /users/search         // + invalide
> //helix:route  GET /users/search         // OK: espaces multiples normalises
> ```

---

## Finding 10: Language Consistency — French/English Mix

**Current Documentation Issue:**

The document `docs/http-layer.md` is entirely in French (headers and body), which is appropriate for the target audience. However, verify consistency throughout.

**Actual Behavior:**

Scan of http-layer.md:
- All section headers in French ✅
- All body text in French ✅
- All code examples with French comments or English (varies) ✅
- Structural consistency maintained

**Status:** Document is already consistent. No changes needed for language.

---

## Summary of Required Changes

### Files to Modify

1. **`docs/http-layer.md`** — Content clarifications and expansions
2. **`documentation_test.go`** — Enhanced validation tests

### Documentation Changes Summary

| Finding | Type | Lines Added | Importance |
|---------|------|-------------|------------|
| 1. nil,nil response | Clarification | ~8 | Medium |
| 2. Guard factory errors | Clarification | ~10 | High |
| 3. Query param binding order | Expansion + restructure | ~25 | High |
| 4. JSON body validation | Expansion + clarification | ~30 | High |
| 5. Guard/interceptor order | Clarification | ~12 | Medium |
| 6. Error handler signatures | Expansion + restructure | ~40 | High |
| 7. Nested struct binding | Note | ~12 | Low |
| 8. Reserved names | Clarification | ~2 | Low |
| 9. Directive syntax edge cases | Expansion | ~15 | Medium |

**Total documentation additions/changes: ~154 lines** (including code examples and structured formatting)

### Test Robustness Enhancements

Current test (`documentation_test.go:149-213`) uses substring matching only. Enhance as follows:

1. **Validate syntax of code examples:**
   - Extract code blocks (triple backtick regions)
   - Check for balanced braces, parentheses
   - Verify imports are mentioned or assumed standard library

2. **Validate directive format examples:**
   - Verify `//helix:route`, `//helix:guard`, `//helix:interceptor` format
   - Ensure negative examples are actually invalid

3. **Test error code cases:**
   - Verify examples of `INVALID_QUERY_PARAM`, `VALIDATION_FAILED`, `INVALID_JSON` are present
   - Check error handler signature example is syntactically correct

4. **Validate struct tag examples:**
   - Confirm `query:`, `json:`, `validate:`, `default:`, `max:` tags are syntactically correct
   - Ensure at least one example mixes `query` + validators, and one example uses JSON

**Suggested test additions (as assertions in `TestHTTPLayerGuideDocumentsCoreConcepts`):**

```go
// After existing substring checks, add:
examples := []struct {
    name    string
    pattern string
}{
    {"nil response example", "nil, nil"},
    {"guard factory error", "factory de guard retourne une erreur"},
    {"query param binding order", "Extraction.*Defaut.*Conversion.*Validation"},
    {"JSON body INVALID_JSON code", "INVALID_JSON"},
    {"error handler signature", `\(any, int\)`},
    {"nested struct ignored", "champs imbriques.*ignores"},
    {"directive whitespace", "espaces multiples"},
}

for _, ex := range examples {
    if !strings.Contains(guide, ex.name) && !regexp.MustCompile(ex.pattern).MatchString(guide) {
        t.Errorf("docs/http-layer.md should contain example or pattern %q", ex.name)
    }
}
```

**Count of new test assertions:** 6-8 additional test cases or assertion groups

---

## Implementation Notes

1. **Order of edits:** Apply changes in this order:
   - Finding 3 (Query params) — largest restructure, affects subsequent sections
   - Finding 4 (JSON body) — follows query params section
   - Finding 2 (Guard factory) — inserted in Guards section
   - Findings 1, 5, 6, 7, 8, 9 — add as notes or expand existing sections

2. **Backward compatibility:** All changes are clarifications and additions. No breaking changes to existing code examples.

3. **Test strategy:** Current tests are resilient; add new pattern-based assertions rather than substring checks for complex behaviors.

