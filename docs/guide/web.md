# Web & Routing

Helix's HTTP layer is built on [Fiber](https://gofiber.io/) and provides convention-based routing, automatic request binding, response mapping, guards, interceptors, and centralized error handling.

## Controllers

A controller is any struct that embeds `helix.Controller`. Helix auto-registers it to the router when passed to `helix.Run` or `web.RegisterController`.

```go
type ProductController struct {
    helix.Controller
    Svc *ProductService `inject:"true"`
}
```

The base route is derived from the struct name by dropping the `Controller` suffix and lowercasing:
`ProductController` → `/products`

Override with a struct tag:

```go
type ProductController struct {
    helix.Controller `helix:"route:/api/v1/products"`
    Svc *ProductService `inject:"true"`
}
```

## Convention Routing

Name methods after REST conventions and routes are registered automatically:

| Method | HTTP route | Status |
|--------|-----------|--------|
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

## Custom Routes

Use `//helix:route METHOD /path` directives for routes outside the convention:

```go
//helix:route POST /auth/login
func (c *AuthController) Login(ctx web.Context, req LoginRequest) (LoginResponse, error) {
    return c.AuthSvc.Authenticate(req.Username, req.Password)
}

//helix:route GET /products/featured
func (c *ProductController) Featured() []Product {
    return c.Svc.GetFeatured()
}

//helix:route DELETE /products/batch
func (c *ProductController) BatchDelete(ctx web.Context, req BatchDeleteRequest) error {
    return c.Svc.BatchDelete(req.IDs)
}
```

## The Context

`web.Context` gives access to the request and response:

```go
// Request
ctx.Method()         // "GET", "POST", ...
ctx.Path()           // "/products/42"
ctx.OriginalURL()    // "/products/42?page=1"
ctx.Param("id")      // route parameter
ctx.Query("page")    // query string parameter
ctx.Header("X-Trace-ID")
ctx.IP()
ctx.Body()           // raw []byte

// Response
ctx.Status(201)
ctx.SetHeader("X-Request-ID", id)
ctx.AppendHeader("Vary", "Accept-Encoding")
ctx.Send([]byte("ok"))
ctx.JSON(map[string]any{"status": "ok"})

// Locals (request-scoped storage, set by guards/interceptors)
ctx.Locals("user")              // read
ctx.Locals("user", userClaims) // write
```

## Request Binding

### Query Parameters

```go
type ListProductsQuery struct {
    Page     int    `query:"page"`
    PageSize int    `query:"pageSize"`
    Category string `query:"category"`
}

//helix:route GET /products/search
func (c *ProductController) Search(ctx web.Context, q ListProductsQuery) ([]Product, error) {
    // q is automatically populated from ?page=1&pageSize=20&category=electronics
    return c.Svc.Search(q.Category, q.Page, q.PageSize)
}
```

### JSON Body

Method parameters after `web.Context` with a struct type are bound from the request body:

```go
type CreateProductInput struct {
    Name        string  `json:"name"        validate:"required,min=2,max=100"`
    Price       float64 `json:"price"       validate:"required,gt=0"`
    CategoryID  int     `json:"categoryId"  validate:"required"`
    Description string  `json:"description"`
}

func (c *ProductController) Create(ctx web.Context, input CreateProductInput) (Product, error) {
    // input is bound and validated automatically
    // returns 400 with field errors if validation fails
    return c.Svc.Create(input)
}
```

Validation uses [go-playground/validator](https://github.com/go-playground/validator). All standard validation tags are supported.

### Validation Error Response

```json
{
  "errors": [
    { "type": "ValidationError", "field": "price",      "message": "must be greater than 0" },
    { "type": "ValidationError", "field": "categoryId", "message": "required" }
  ]
}
```

## Guards

Guards protect routes from unauthorized access. They run before the handler.

### Built-in auth guard

```go
//helix:guard auth
func (c *APIController) Profile(ctx web.Context) (UserProfile, error) {
    // only reachable with a valid JWT Bearer token
    claims, _ := ctx.Locals("claims").(map[string]any)
    return c.UserSvc.GetProfile(claims["sub"].(string))
}
```

### Role guard

```go
//helix:guard role:admin
func (c *AdminController) Users(_ web.Context) ([]User, error) {
    return c.UserSvc.ListAll()
}
```

### Custom guard

```go
type RateLimitGuard struct{}

func (g *RateLimitGuard) CanActivate(ctx web.Context) error {
    if isRateLimited(ctx.IP()) {
        return web.NewRequestError(http.StatusTooManyRequests, "rate limit exceeded")
    }
    return nil
}

// Register:
web.RegisterGuard(server, "rate-limit", &RateLimitGuard{})

// Use:
//helix:guard rate-limit
func (c *APIController) Search(...) { ... }
```

### Global guard

Apply a guard to every route:

```go
web.ApplyGlobalGuard(server, jwtGuard)
```

## Interceptors

Interceptors wrap handlers — useful for caching, logging, tracing, or request transformation.

```go
type AuditInterceptor struct{}

func (i *AuditInterceptor) Intercept(ctx web.Context, next web.HandlerFunc) error {
    start := time.Now()
    err := next(ctx)
    slog.Info("request", "method", ctx.Method(), "path", ctx.Path(),
        "duration", time.Since(start), "error", err)
    return err
}

// Register:
web.RegisterInterceptor(server, "audit", &AuditInterceptor{})

// Use:
//helix:interceptor audit
func (c *ProductController) Create(...) { ... }
```

## Error Handling

### Returning errors from handlers

Helix maps returned errors to HTTP status codes automatically:

```go
func (c *ProductController) Show(ctx web.Context) (Product, error) {
    id, _ := strconv.Atoi(ctx.Param("id"))
    p, ok := c.Svc.Get(id)
    if !ok {
        return Product{}, web.NewRequestError(http.StatusNotFound, "product not found")
    }
    return p, nil
}
```

```json
// 404 response
{
  "error": {
    "type": "NotFound",
    "message": "product not found"
  }
}
```

### Custom error handlers

```go
type DomainErrorHandler struct {
    helix.ErrorHandler
}

//helix:handles OutOfStockError
func (h *DomainErrorHandler) HandleOutOfStock(err OutOfStockError) (any, int) {
    return map[string]any{
        "error":   "OUT_OF_STOCK",
        "product": err.ProductID,
    }, http.StatusConflict
}

// Register:
web.RegisterErrorHandler(server, &DomainErrorHandler{})
```

## Helper Errors

```go
web.Unauthorized("invalid credentials")      // 401
web.Forbidden("insufficient permissions")    // 403
web.NewRequestError(404, "not found")        // any 4xx
```

## Handler return types

Helix inspects handler return values to decide the HTTP response:

| Return signature | Body | Default status |
|-----------------|------|---------------|
| `func() error` | empty | 204 (or error status) |
| `func() (T, error)` | JSON-encoded T | 200 |
| `Create()` method name | JSON-encoded T | 201 |
| `Delete()` method name | empty | 204 |
| `func(ctx web.Context)` | set via `ctx.JSON()`/`ctx.Send()` | whatever you set |
| returns `(nil, nil)` | empty | 204 |
| explicit `ctx.Status(n)` | overrides default | n |

```go
// 200 with JSON body:
func (c *ProductController) Show(ctx web.Context) (Product, error) { ... }

// 201 Created (convention):
func (c *ProductController) Create(ctx web.Context, in Input) (Product, error) { ... }

// 204 No Content:
func (c *ProductController) Delete(ctx web.Context) error { ... }

// Custom status override:
func (c *OrderController) Checkout(ctx web.Context, in CheckoutInput) (*Order, error) {
    order, err := c.Svc.Checkout(in)
    if err != nil { return nil, err }
    ctx.Status(http.StatusAccepted)  // 202 instead of 200
    return order, nil
}
```

## Multiple guards

Stack multiple `//helix:guard` directives on the same method — all must pass:

```go
//helix:guard auth
//helix:guard rate-limit
//helix:guard role:admin
func (c *AdminController) Export(_ web.Context) (ExportData, error) {
    // must: have valid JWT AND not be rate-limited AND have admin role
    return c.Svc.Export()
}
```

Guards run in declaration order. The first to return an error short-circuits the chain.

## Interceptor ordering

When multiple interceptors are stacked, they wrap each other like onion layers — `before` code runs in declaration order, `after` code runs in reverse:

```go
//helix:interceptor log
//helix:interceptor cache
func (c *ProductController) Index() []Product { ... }
```

Execution order:
```
log.before → cache.before → handler → cache.after → log.after
```

```go
type LogInterceptor struct{}

func (i *LogInterceptor) Intercept(ctx web.Context, next web.HandlerFunc) error {
    start := time.Now()
    err := next(ctx)  // ← runs cache interceptor + handler
    slog.Info("request", "path", ctx.Path(), "duration", time.Since(start), "err", err)
    return err
}
```

## Cache interceptor

Helix includes a built-in cache interceptor for GET responses:

```go
// Register with TTL:
web.RegisterInterceptorFactory(server, "cache", web.NewCacheInterceptorFactory(
    web.CacheOptions{DefaultTTL: 60 * time.Second},
))

//helix:interceptor cache
func (c *ProductController) Index() []Product { ... }

// Custom TTL per route:
//helix:interceptor cache:300   // 5 minutes
func (c *ProductController) Show(ctx web.Context) (Product, error) { ... }
```

Cache keys are computed from `method + path + query string`. Cache is invalidated automatically on non-GET requests to the same path prefix.

## Nested struct binding

Query parameters and JSON bodies support nested structs:

```go
type SearchFilter struct {
    MinPrice float64 `query:"minPrice"`
    MaxPrice float64 `query:"maxPrice"`
    InStock  bool    `query:"inStock"`
}

type SearchQuery struct {
    Query    string       `query:"q"`
    Page     int          `query:"page"`
    PageSize int          `query:"pageSize"`
    Filter   SearchFilter `query:"filter"`  // bound from filter.minPrice, filter.maxPrice, etc.
}

//helix:route GET /products/search
func (c *ProductController) Search(ctx web.Context, q SearchQuery) ([]Product, error) {
    // ?q=keyboard&page=1&filter.minPrice=50&filter.maxPrice=200&filter.inStock=true
    return c.Svc.Search(q)
}
```

JSON body nesting works the same way with standard `json:"..."` tags.

## File upload

Handle multipart form data using `ctx.Body()` or Fiber's form file API directly:

```go
//helix:route POST /uploads
func (c *FileController) Upload(ctx web.Context) (*UploadResult, error) {
    fiberCtx := ctx.Raw()  // access the underlying *fiber.Ctx

    file, err := fiberCtx.FormFile("file")
    if err != nil {
        return nil, web.NewRequestError(http.StatusBadRequest, "file is required")
    }

    if file.Size > 10<<20 { // 10 MB
        return nil, web.NewRequestError(http.StatusRequestEntityTooLarge, "file exceeds 10 MB limit")
    }

    result, err := c.StorageSvc.Store(file)
    if err != nil {
        return nil, err
    }
    return result, nil
}
```

## Route observer

Observe route execution for custom metrics, logging, or tracing:

```go
type MyObserver struct{}

func (o *MyObserver) Observe(obs web.RouteObservation) {
    // obs.Method, obs.Path, obs.StatusCode, obs.Duration, obs.Error
    myMetrics.With(obs.Method, obs.Path, strconv.Itoa(obs.StatusCode)).Inc()
}

server := web.NewServer(
    web.WithRouteObserver(&MyObserver{}),
)
```

Helix's built-in Prometheus metrics observer uses this same hook — see [Observability](/guide/observability).

## Manual Server Usage

You can also use the HTTP server directly without `helix.Run`:

```go
server := web.NewServer(
    web.WithRouteObserver(metricsObserver),
    web.WithTracerProvider(tp),
)

web.RegisterController(server, &ProductController{})
web.ApplyGlobalGuard(server, jwtGuard)

server.Start(":8080")
```
