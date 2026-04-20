package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// testTimeout is the default timeout in milliseconds used by ServeHTTP.
const testTimeout = 30_000

// Context mirrors the public web.Context contract without importing web and
// creating a package cycle.
type Context interface {
	Method() string
	OriginalURL() string
	Param(key string) string
	Query(key string) string
	Header(key string) string
	IP() string
	Body() []byte
	Status(code int)
	SetHeader(key, value string)
	Send(body []byte) error
	JSON(body any) error
}

// HandlerFunc handles a request through the internal adapter.
type HandlerFunc func(Context) error

// Adapter is the internal contract that the public web package uses.
type Adapter interface {
	Start(addr string) error
	Stop(ctx context.Context) error
	RegisterRoute(method, path string, handler HandlerFunc) error
	ServeHTTP(req *http.Request) (*http.Response, error)
}

// AdapterOption configures a fiberAdapter.
type AdapterOption func(*adapterOptions)

type adapterOptions struct {
	tracerProvider trace.TracerProvider
}

// WithTracerProvider installs an OTel TracerProvider in the adapter.
// When non-nil, a tracing middleware is added before all routes.
func WithTracerProvider(tp trace.TracerProvider) AdapterOption {
	return func(o *adapterOptions) {
		o.tracerProvider = tp
	}
}

// fiberAdapter adapts Fiber to Helix's public HTTP abstractions.
type fiberAdapter struct {
	app *fiber.App
}

// NewAdapter creates a Fiber-backed adapter.
func NewAdapter(opts ...AdapterOption) Adapter {
	o := &adapterOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}

	app := fiber.New()

	if o.tracerProvider != nil {
		app.Use(tracingMiddleware(o.tracerProvider))
	}

	return &fiberAdapter{app: app}
}

// tracingMiddleware returns a Fiber middleware that creates an OTel span for
// every request and propagates the W3C trace context.
//
// Span name is set to "METHOD /route/:param" after routing completes so that
// parametrised paths (e.g. /users/:id) are used instead of raw URLs, which
// prevents cardinality explosion in tracing backends.
func tracingMiddleware(tp trace.TracerProvider) fiber.Handler {
	tracer := tp.Tracer("helix.web")
	// Use a fixed W3C propagator so the middleware works independently of
	// whether the caller has set a global TextMapPropagator via otel.SetTextMapPropagator.
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	return func(c *fiber.Ctx) error {
		// Extract incoming trace context from request headers.
		carrier := fiberHeaderCarrier{ctx: c}
		ctx := propagator.Extract(c.UserContext(), carrier)

		// Start span with a temporary name (route not resolved yet).
		ctx, span := tracer.Start(ctx, c.Method()+" "+c.OriginalURL())
		defer span.End()
		c.SetUserContext(ctx)

		err := c.Next()

		// Update span name with the matched route pattern after routing.
		if route := c.Route(); route != nil {
			span.SetName(c.Method() + " " + route.Path)
		}

		// Inject outgoing trace context into response headers.
		propagator.Inject(ctx, carrier)

		return err
	}
}

// fiberHeaderCarrier implements propagation.TextMapCarrier for Fiber contexts.
// It reads from request headers and writes to response headers.
type fiberHeaderCarrier struct {
	ctx *fiber.Ctx
}

func (c fiberHeaderCarrier) Get(key string) string {
	return string(c.ctx.Request().Header.Peek(key))
}

func (c fiberHeaderCarrier) Set(key, value string) {
	c.ctx.Set(key, value)
}

// Keys returns nil — the W3C TraceContext propagator does not call Keys for extraction.
func (c fiberHeaderCarrier) Keys() []string {
	return nil
}

// Start begins listening on addr.
func (a *fiberAdapter) Start(addr string) error {
	return a.app.Listen(addr)
}

// Stop gracefully shuts down the underlying server.
func (a *fiberAdapter) Stop(ctx context.Context) error {
	return a.app.ShutdownWithContext(ctx)
}

// RegisterRoute registers a route on the underlying Fiber app.
func (a *fiberAdapter) RegisterRoute(method, path string, handler HandlerFunc) error {
	fiberHandler := func(ctx *fiber.Ctx) error {
		return handler(fiberContext{ctx: ctx})
	}

	switch method {
	case http.MethodGet:
		a.app.Get(path, fiberHandler)
	case http.MethodPost:
		a.app.Post(path, fiberHandler)
	case http.MethodPut:
		a.app.Put(path, fiberHandler)
	case http.MethodPatch:
		a.app.Patch(path, fiberHandler)
	case http.MethodDelete:
		a.app.Delete(path, fiberHandler)
	default:
		return fmt.Errorf("unsupported method %q", method)
	}
	return nil
}

// ServeHTTP executes a request without starting a network listener.
func (a *fiberAdapter) ServeHTTP(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("web: serve http: nil request")
	}
	return a.app.Test(req, testTimeout)
}

type fiberContext struct {
	ctx *fiber.Ctx
}

func (c fiberContext) Method() string {
	return c.ctx.Method()
}

func (c fiberContext) OriginalURL() string {
	return c.ctx.OriginalURL()
}

func (c fiberContext) Param(key string) string {
	return c.ctx.Params(key)
}

func (c fiberContext) Query(key string) string {
	return c.ctx.Query(key)
}

func (c fiberContext) Header(key string) string {
	return c.ctx.Get(key)
}

func (c fiberContext) IP() string {
	return c.ctx.IP()
}

func (c fiberContext) Body() []byte {
	body := c.ctx.BodyRaw()
	copied := make([]byte, len(body))
	copy(copied, body)
	return copied
}

func (c fiberContext) Status(code int) {
	c.ctx.Status(code)
}

func (c fiberContext) SetHeader(key, value string) {
	c.ctx.Set(key, value)
}

func (c fiberContext) Send(body []byte) error {
	return c.ctx.Send(body)
}

func (c fiberContext) JSON(body any) error {
	return c.ctx.JSON(body)
}
