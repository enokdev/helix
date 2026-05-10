# Error Handling

Helix provides a layered error handling system: automatic mapping of returned errors to HTTP status codes, structured error responses, validation error formatting, and centralized domain error handlers.

## How errors become HTTP responses

When a handler returns an error, Helix inspects it in order:

1. Does it implement `web.RequestError`? → use its `StatusCode()`, serialize as structured JSON
2. Is it a validation error (produced by binding)? → 400 with field-level details
3. Is there a registered `ErrorHandler` that handles this type? → delegate to it
4. Otherwise → 500 Internal Server Error (message hidden in production)

```
handler returns error
        │
        ▼
implements RequestError? ──yes──▶ JSON error response with its StatusCode
        │ no
        ▼
validation error? ──yes──▶ 400 with per-field errors array
        │ no
        ▼
registered ErrorHandler matches? ──yes──▶ delegate to handler
        │ no
        ▼
500 Internal Server Error
```

## The `RequestError` interface

```go
type RequestError interface {
    error
    StatusCode() int    // HTTP status code (400, 401, 403, 404, ...)
    ErrorType() string  // human-readable type name ("NotFound", "Unauthorized")
    ErrorCode() string  // machine-readable code ("USER_NOT_FOUND")
    ErrorField() string // field that caused the error (for field-level errors)
}
```

Any error implementing this interface is serialized as:

```json
{
  "error": {
    "type":    "NotFound",
    "code":    "USER_NOT_FOUND",
    "message": "user 42 not found"
  }
}
```

## Helper constructors

### `web.NewRequestError`

General-purpose HTTP error:

```go
// 404
return nil, web.NewRequestError(http.StatusNotFound, "user not found")

// 400
return nil, web.NewRequestError(http.StatusBadRequest, "invalid page parameter")

// 409
return nil, web.NewRequestError(http.StatusConflict, "email already registered")
```

### `web.Unauthorized` / `web.Forbidden`

```go
return nil, web.Unauthorized("missing or invalid token")   // 401
return nil, web.Forbidden("insufficient permissions")      // 403
```

### `web.NewFieldError`

Error tied to a specific field (useful in services, complements binding validation):

```go
return nil, web.NewFieldError("email", "email is already taken")
// → 400 with { "error": { "type": "ValidationError", "field": "email", "message": "..." } }
```

## Binding and validation errors

When Helix binds the request body or query parameters and validation fails, it returns 400 automatically with a structured array of field errors:

```json
{
  "errors": [
    { "type": "ValidationError", "field": "email",    "message": "must be a valid email" },
    { "type": "ValidationError", "field": "password", "message": "min length is 8" }
  ]
}
```

These are produced before your handler is called — you don't need to handle them yourself.

Validation tags come from [go-playground/validator](https://github.com/go-playground/validator):

```go
type RegisterInput struct {
    Email    string `json:"email"    validate:"required,email"`
    Password string `json:"password" validate:"required,min=8,max=72"`
    Age      int    `json:"age"      validate:"gte=18"`
}
```

## Custom error types

Define your own domain errors that implement `RequestError`:

```go
type ErrOutOfStock struct {
    ProductID uint
    Requested int
    Available int
}

func (e ErrOutOfStock) Error() string {
    return fmt.Sprintf("product %d: requested %d but only %d available", e.ProductID, e.Requested, e.Available)
}

func (e ErrOutOfStock) StatusCode() int    { return http.StatusConflict }
func (e ErrOutOfStock) ErrorType() string  { return "OutOfStock" }
func (e ErrOutOfStock) ErrorCode() string  { return "PRODUCT_OUT_OF_STOCK" }
func (e ErrOutOfStock) ErrorField() string { return "" }
```

Return it directly from any handler or service:

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

Response (409 Conflict):

```json
{
  "error": {
    "type":    "OutOfStock",
    "code":    "PRODUCT_OUT_OF_STOCK",
    "message": "product 7: requested 5 but only 2 available"
  }
}
```

## Centralized error handlers

For errors you don't control (third-party library errors, `errors.New(...)`) or when you want a single place for all domain → HTTP mappings, implement `helix.ErrorHandler`:

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
            "message": "a record with this value already exists",
        }}, http.StatusConflict
    case "23503": // foreign_key_violation
        return map[string]any{"error": map[string]any{
            "type":    "ReferenceNotFound",
            "message": "referenced record does not exist",
        }}, http.StatusUnprocessableEntity
    default:
        return map[string]any{"error": "database error"}, http.StatusInternalServerError
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

Register as a component:

```go
helix.Run(helix.App{
    Components: []any{
        &DomainErrorHandler{},
        // ...
    },
})
```

The `//helix:handles TypeName` directive tells Helix which error type this method handles. The method signature is:

```go
func (h *Handler) HandleXxx(err YourErrorType) (responseBody any, statusCode int)
```

## Domain error → HTTP mapping pattern

A common pattern is a single switch in the service layer that wraps domain errors:

```go
func domainToHTTP(err error) error {
    switch {
    case errors.Is(err, ErrUserNotFound):
        return web.NewRequestError(http.StatusNotFound, "user not found")
    case errors.Is(err, ErrEmailTaken):
        return web.NewFieldError("email", "email is already registered")
    case errors.Is(err, ErrWeakPassword):
        return web.NewFieldError("password", "password does not meet requirements")
    default:
        return err // let Helix handle as 500
    }
}

// In a handler:
func (c *UserController) Create(ctx web.Context, in CreateInput) (*User, error) {
    user, err := c.Svc.Create(ctx, in)
    if err != nil {
        return nil, domainToHTTP(err)
    }
    ctx.Status(http.StatusCreated)
    return user, nil
}
```

## Handler return types and status codes

| Return signature | Success status | Error handling |
|-----------------|---------------|----------------|
| `() error` | 204 No Content | standard |
| `() (T, error)` | 200 OK | standard |
| `Create()` method convention | 201 Created | standard |
| `Delete()` method convention | 204 No Content | standard |
| explicit `ctx.Status(n)` | n | standard |
| `(nil, nil)` | 204 No Content | no body |

## Panic recovery

Helix catches panics in handlers. A panic produces a 500 response without crashing the server:

```json
{
  "error": {
    "type":    "InternalServerError",
    "message": "an unexpected error occurred"
  }
}
```

The full stack trace is logged at ERROR level. The panic value is never exposed to the client.

## Error logging

All 5xx errors are logged automatically:

```json
{"level":"ERROR","method":"POST","path":"/orders","status":500,"error":"sql: no rows in result set","latency_ms":12}
```

4xx errors are logged at DEBUG level (they are expected client errors, not application problems).

## Summary

| Scenario | What to do |
|----------|-----------|
| Not found | `return nil, web.NewRequestError(404, "...")` |
| Auth failure | `return nil, web.Unauthorized("...")` |
| Permission denied | `return nil, web.Forbidden("...")` |
| Validation (business rule) | `return nil, web.NewFieldError("field", "...")` |
| Domain error (you own it) | implement `RequestError` interface |
| Third-party error | register `ErrorHandler` with `//helix:handles` |
| Unexpected error | return it — Helix maps to 500 |
