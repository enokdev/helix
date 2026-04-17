package web_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	helix "github.com/enokdev/helix"
	"github.com/enokdev/helix/web"
)

func TestRegisterController_RegistersConventionalRoutes(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	controller := &UserController{calls: make(map[string]string)}

	if err := web.RegisterController(server, controller); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	tests := []struct {
		name   string
		method string
		path   string
		call   string
		wantID string
	}{
		{name: "index", method: http.MethodGet, path: "/users", call: "index"},
		{name: "show", method: http.MethodGet, path: "/users/42", call: "show", wantID: "42"},
		{name: "create", method: http.MethodPost, path: "/users", call: "create"},
		{name: "update", method: http.MethodPut, path: "/users/42", call: "update", wantID: "42"},
		{name: "delete", method: http.MethodDelete, path: "/users/42", call: "delete", wantID: "42"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.ServeHTTP(httptest.NewRequest(tt.method, tt.path, nil))
			if err != nil {
				t.Fatalf("ServeHTTP() error = %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if got := controller.calls[tt.call]; got != valueOrCalled(tt.wantID) {
				t.Fatalf("controller call %q = %q, want %q", tt.call, got, valueOrCalled(tt.wantID))
			}
		})
	}
}

func TestRegisterController_DerivesRoutePrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		controller any
		path       string
	}{
		{name: "blog post", controller: &BlogPostController{called: make(map[string]bool)}, path: "/blog-posts"},
		{name: "category y suffix", controller: &CategoryController{called: make(map[string]bool)}, path: "/categories"},
		{name: "pointer marker", controller: &PointerMarkerController{}, path: "/pointer-markers"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t)
			if err := web.RegisterController(server, tt.controller); err != nil {
				t.Fatalf("RegisterController() error = %v", err)
			}

			resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, tt.path, nil))
			if err != nil {
				t.Fatalf("ServeHTTP() error = %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
			}
		})
	}
}

func TestRegisterController_SupportsSimpleHandlerSignatures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{name: "func no args no returns", method: http.MethodGet, path: "/signatures", want: "index"},
		{name: "func no args error", method: http.MethodPost, path: "/signatures", want: "create"},
		{name: "func context no returns", method: http.MethodPut, path: "/signatures/abc", want: "abc"},
		{name: "func context error via show", method: http.MethodGet, path: "/signatures/abc", want: "abc"},
		{name: "func context error via delete", method: http.MethodDelete, path: "/signatures/xyz", want: "xyz"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &SignatureController{}
			srv := newTestServer(t)
			if err := web.RegisterController(srv, c); err != nil {
				t.Fatalf("RegisterController() error = %v", err)
			}

			resp, err := srv.ServeHTTP(httptest.NewRequest(tt.method, tt.path, nil))
			if err != nil {
				t.Fatalf("ServeHTTP() error = %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if c.last != tt.want {
				t.Fatalf("last call = %q, want %q", c.last, tt.want)
			}
		})
	}
}

func TestRegisterController_RejectsInvalidController(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		controller any
	}{
		{name: "nil", controller: nil},
		{name: "non pointer", controller: UserController{}},
		{name: "nil pointer", controller: (*UserController)(nil)},
		{name: "missing marker", controller: &UnmarkedController{}},
		{name: "missing controller suffix", controller: &Resource{}},
		{name: "no conventional methods", controller: &EmptyController{}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := web.RegisterController(newTestServer(t), tt.controller)
			if !errors.Is(err, web.ErrInvalidController) {
				t.Fatalf("RegisterController() error = %v, want ErrInvalidController", err)
			}
		})
	}
}

func TestRegisterController_RejectsUnsupportedMethodSignature(t *testing.T) {
	t.Parallel()

	err := web.RegisterController(newTestServer(t), &UnsupportedSignatureController{})
	if !errors.Is(err, web.ErrUnsupportedHandler) {
		t.Fatalf("RegisterController() error = %v, want ErrUnsupportedHandler", err)
	}
}

func TestRegisterController_RegistersCustomRouteDirective(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	controller := &CustomRouteController{}

	if err := web.RegisterController(server, controller); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/custom-routes/search", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if controller.last != "search" {
		t.Fatalf("last call = %q, want %q", controller.last, "search")
	}
}

func TestRegisterController_RegistersMultipleCustomRouteDirectives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		want   string
	}{
		{name: "get", method: http.MethodGet, want: "get"},
		{name: "post", method: http.MethodPost, want: "post"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t)
			controller := &CustomRouteController{}

			if err := web.RegisterController(server, controller); err != nil {
				t.Fatalf("RegisterController() error = %v", err)
			}

			req := httptest.NewRequest(tt.method, "/custom-routes/search", nil)
			req.Header.Set("X-Method", tt.want)
			resp, err := server.ServeHTTP(req)
			if err != nil {
				t.Fatalf("ServeHTTP() error = %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if controller.last != tt.want {
				t.Fatalf("last call = %q, want %q", controller.last, tt.want)
			}
		})
	}
}

func TestRegisterController_PrioritizesStaticCustomRoutesBeforeParameterizedConventions(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	controller := &CustomRouteController{}

	if err := web.RegisterController(server, controller); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/custom-routes/search", nil)
	req.Header.Set("X-Method", "get")
	resp, err := server.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if controller.last != "get" {
		t.Fatalf("last call = %q, want %q; custom static route should not be captured by Show", controller.last, "get")
	}
}

func TestRegisterController_IgnoresUnroutedMethods(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	if err := web.RegisterController(server, &UnroutedMethodController{}); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/unrouted-methods/helper", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestRegisterController_RejectsMalformedRouteDirective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		controller any
	}{
		{name: "space after slashes", controller: &SpaceDirectiveController{}},
		{name: "plus prefix", controller: &PlusDirectiveController{}},
		{name: "missing path", controller: &MissingPathDirectiveController{}},
		{name: "too many tokens", controller: &TooManyTokensDirectiveController{}},
		{name: "unsupported method", controller: &UnsupportedMethodDirectiveController{}},
		{name: "invalid path", controller: &InvalidPathDirectiveController{}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := web.RegisterController(newTestServer(t), tt.controller)
			if !errors.Is(err, web.ErrInvalidDirective) {
				t.Fatalf("RegisterController() error = %v, want ErrInvalidDirective", err)
			}
		})
	}
}

func TestRegisterController_DoesNotPartiallyRegisterRoutesWhenDirectiveInvalid(t *testing.T) {
	t.Parallel()

	server := &recordingServer{}
	err := web.RegisterController(server, &SpaceDirectiveController{})
	if !errors.Is(err, web.ErrInvalidDirective) {
		t.Fatalf("RegisterController() error = %v, want ErrInvalidDirective", err)
	}
	if len(server.routes) != 0 {
		t.Fatalf("registered routes = %v, want none", server.routes)
	}
}

func TestRegisterController_InjectsTypedQueryParams(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	controller := &TypedQueryController{}

	if err := web.RegisterController(server, controller); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/typed-queries?page_size=50&email=alice@example.com", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !controller.called {
		t.Fatal("handler was not called")
	}
	if controller.query.Page != 1 {
		t.Fatalf("Page = %d, want default %d", controller.query.Page, 1)
	}
	if controller.query.PageSize != 50 {
		t.Fatalf("PageSize = %d, want %d", controller.query.PageSize, 50)
	}
	if controller.query.Email != "alice@example.com" {
		t.Fatalf("Email = %q, want %q", controller.query.Email, "alice@example.com")
	}
}

func TestRegisterController_RejectsInvalidTypedQueryParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		query     string
		wantCode  string
		wantField string
	}{
		{name: "missing required email", query: "page_size=20", wantCode: "VALIDATION_FAILED", wantField: "email"},
		{name: "invalid email", query: "page_size=20&email=not-an-email", wantCode: "VALIDATION_FAILED", wantField: "email"},
		{name: "max exceeded", query: "page_size=101&email=alice@example.com", wantCode: "VALIDATION_FAILED", wantField: "page_size"},
		{name: "invalid int", query: "page=abc&page_size=20&email=alice@example.com", wantCode: "INVALID_QUERY_PARAM", wantField: "page"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t)
			controller := &TypedQueryController{}

			if err := web.RegisterController(server, controller); err != nil {
				t.Fatalf("RegisterController() error = %v", err)
			}

			resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/typed-queries?"+tt.query, nil))
			if err != nil {
				t.Fatalf("ServeHTTP() error = %v", err)
			}
			defer resp.Body.Close()

			assertErrorResponse(t, resp, http.StatusBadRequest, tt.wantCode, tt.wantField)
			if controller.called {
				t.Fatal("handler should not be called when query extraction or validation fails")
			}
		})
	}
}

func TestRegisterController_InjectsJSONBody(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	controller := &BodyBindingController{}

	if err := web.RegisterController(server, controller); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/body-bindings", strings.NewReader(`{"name":"Alice","email":"alice@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := server.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !controller.called {
		t.Fatal("handler was not called")
	}
	if controller.body.Name != "Alice" {
		t.Fatalf("Name = %q, want %q", controller.body.Name, "Alice")
	}
	if controller.body.Email != "alice@example.com" {
		t.Fatalf("Email = %q, want %q", controller.body.Email, "alice@example.com")
	}
}

func TestRegisterController_RejectsInvalidJSONBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		wantCode  string
		wantField string
	}{
		{name: "empty body", body: "", wantCode: "INVALID_JSON"},
		{name: "invalid json", body: `{"name":`, wantCode: "INVALID_JSON"},
		{name: "unknown field", body: `{"name":"Alice","email":"alice@example.com","role":"admin"}`, wantCode: "INVALID_JSON", wantField: "role"},
		{name: "invalid field type", body: `{"name":42,"email":"alice@example.com"}`, wantCode: "INVALID_JSON", wantField: "name"},
		{name: "missing required email", body: `{"name":"Alice"}`, wantCode: "VALIDATION_FAILED", wantField: "email"},
		{name: "invalid email", body: `{"name":"Alice","email":"not-an-email"}`, wantCode: "VALIDATION_FAILED", wantField: "email"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t)
			controller := &BodyBindingController{}

			if err := web.RegisterController(server, controller); err != nil {
				t.Fatalf("RegisterController() error = %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/body-bindings", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := server.ServeHTTP(req)
			if err != nil {
				t.Fatalf("ServeHTTP() error = %v", err)
			}
			defer resp.Body.Close()

			assertErrorResponse(t, resp, http.StatusBadRequest, tt.wantCode, tt.wantField)
			if controller.called {
				t.Fatal("handler should not be called when body extraction or validation fails")
			}
		})
	}
}

func TestRegisterController_InjectsContextAndTypedQueryParams(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	controller := &MixedBindingController{}

	if err := web.RegisterController(server, controller); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/mixed-bindings/42?trace_id=abc", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if controller.id != "42" {
		t.Fatalf("id = %q, want %q", controller.id, "42")
	}
	if controller.query.TraceID != "abc" {
		t.Fatalf("TraceID = %q, want %q", controller.query.TraceID, "abc")
	}
}

func TestRegisterController_RejectsAmbiguousTypedInput(t *testing.T) {
	t.Parallel()

	err := web.RegisterController(newTestServer(t), &AmbiguousBindingController{})
	if !errors.Is(err, web.ErrUnsupportedHandler) {
		t.Fatalf("RegisterController() error = %v, want ErrUnsupportedHandler", err)
	}
}

func TestRequestErrorMatchesErrInvalidRequest(t *testing.T) {
	t.Parallel()

	if !errors.Is(&web.RequestError{}, web.ErrInvalidRequest) {
		t.Fatal("RequestError should match ErrInvalidRequest")
	}
}

func TestRegisterController_RejectsInvalidMaxTagAtRegistration(t *testing.T) {
	t.Parallel()

	err := web.RegisterController(newTestServer(t), &InvalidMaxTagController{})
	if !errors.Is(err, web.ErrUnsupportedHandler) {
		t.Fatalf("RegisterController() error = %v, want ErrUnsupportedHandler", err)
	}
}

func assertErrorResponse(t *testing.T, resp *http.Response, wantStatus int, wantCode, wantField string) {
	t.Helper()

	if resp.StatusCode != wantStatus {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("StatusCode = %d, want %d; body = %s", resp.StatusCode, wantStatus, string(body))
	}

	var payload struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Field   string `json:"field"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Error.Code != wantCode {
		t.Fatalf("error.code = %q, want %q", payload.Error.Code, wantCode)
	}
	if wantField != "" && payload.Error.Field != wantField {
		t.Fatalf("error.field = %q, want %q", payload.Error.Field, wantField)
	}
	if payload.Error.Type == "" {
		t.Fatal("error.type should not be empty")
	}
	if payload.Error.Message == "" {
		t.Fatal("error.message should not be empty")
	}
}

func valueOrCalled(value string) string {
	if value == "" {
		return "called"
	}
	return value
}

type UserController struct {
	helix.Controller
	calls map[string]string
}

func (c *UserController) Index() {
	c.calls["index"] = "called"
}

func (c *UserController) Show(ctx web.Context) error {
	c.calls["show"] = ctx.Param("id")
	return nil
}

func (c *UserController) Create() error {
	c.calls["create"] = "called"
	return nil
}

func (c *UserController) Update(ctx web.Context) {
	c.calls["update"] = ctx.Param("id")
}

func (c *UserController) Delete(ctx web.Context) error {
	c.calls["delete"] = ctx.Param("id")
	return nil
}

type BlogPostController struct {
	helix.Controller
	called map[string]bool
}

func (c *BlogPostController) Index() {
	c.called["index"] = true
}

type CategoryController struct {
	helix.Controller
	called map[string]bool
}

func (c *CategoryController) Index() {
	c.called["index"] = true
}

type PointerMarkerController struct {
	*helix.Controller
}

func (c *PointerMarkerController) Index() {}

type SignatureController struct {
	helix.Controller
	last string
}

func (c *SignatureController) Index() {
	c.last = "index"
}

func (c *SignatureController) Show(ctx web.Context) error {
	c.last = ctx.Param("id")
	return nil
}

func (c *SignatureController) Create() error {
	c.last = "create"
	return nil
}

func (c *SignatureController) Update(ctx web.Context) {
	c.last = ctx.Param("id")
}

func (c *SignatureController) Delete(ctx web.Context) error {
	c.last = ctx.Param("id")
	return nil
}

type UnmarkedController struct{}

func (c *UnmarkedController) Index() {}

type Resource struct {
	helix.Controller
}

func (c *Resource) Index() {}

type EmptyController struct {
	helix.Controller
}

type UnsupportedSignatureController struct {
	helix.Controller
}

func (c *UnsupportedSignatureController) Index(_ struct{}) error {
	return nil
}

type CustomRouteController struct {
	helix.Controller
	last string
}

func (c *CustomRouteController) Show(ctx web.Context) error {
	c.last = "show:" + ctx.Param("id")
	return nil
}

//helix:route GET /custom-routes/search
//helix:route POST /custom-routes/search
func (c *CustomRouteController) Search(ctx web.Context) error {
	c.last = strings.ToLower(ctx.Header("X-Method"))
	if c.last == "" {
		c.last = strings.ToLower(ctx.Param("method"))
	}
	if c.last == "" {
		c.last = "search"
	}
	return nil
}

type UnroutedMethodController struct {
	helix.Controller
}

func (c *UnroutedMethodController) Index() {}

func (c *UnroutedMethodController) Helper() {}

type SpaceDirectiveController struct {
	helix.Controller
}

func (c *SpaceDirectiveController) Index() {}

// helix:route GET /space-directives/search
func (c *SpaceDirectiveController) Search() {}

type PlusDirectiveController struct {
	helix.Controller
}

// +helix:route GET /plus-directives/search
func (c *PlusDirectiveController) Search() {}

type MissingPathDirectiveController struct {
	helix.Controller
}

//helix:route GET
func (c *MissingPathDirectiveController) Search() {}

type TooManyTokensDirectiveController struct {
	helix.Controller
}

//helix:route GET /too-many-tokens-directives/search extra
func (c *TooManyTokensDirectiveController) Search() {}

type UnsupportedMethodDirectiveController struct {
	helix.Controller
}

//helix:route TRACE /unsupported-method-directives/search
func (c *UnsupportedMethodDirectiveController) Search() {}

type InvalidPathDirectiveController struct {
	helix.Controller
}

//helix:route GET invalid-path-directives/search
func (c *InvalidPathDirectiveController) Search() {}

type TypedQueryController struct {
	helix.Controller
	called bool
	query  typedQueryParams
}

type typedQueryParams struct {
	Page     int    `query:"page" default:"1" validate:"min=1"`
	PageSize int    `query:"page_size" default:"20" max:"100" validate:"min=1"`
	Email    string `query:"email" validate:"required,email"`
}

func (c *TypedQueryController) Index(params typedQueryParams) error {
	c.called = true
	c.query = params
	return nil
}

type BodyBindingController struct {
	helix.Controller
	called bool
	body   createBody
}

type createBody struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

func (c *BodyBindingController) Create(body createBody) error {
	c.called = true
	c.body = body
	return nil
}

type MixedBindingController struct {
	helix.Controller
	id    string
	query mixedQueryParams
}

type mixedQueryParams struct {
	TraceID string `query:"trace_id" validate:"required"`
}

func (c *MixedBindingController) Show(ctx web.Context, params mixedQueryParams) error {
	c.id = ctx.Param("id")
	c.query = params
	return nil
}

type AmbiguousBindingController struct {
	helix.Controller
}

type ambiguousInput struct {
	ID string `query:"id" json:"id"`
}

func (c *AmbiguousBindingController) Index(input ambiguousInput) error {
	return nil
}

type InvalidMaxTagController struct {
	helix.Controller
}

type invalidMaxTagParams struct {
	Page int `query:"page" max:"abc"`
}

func (c *InvalidMaxTagController) Index(params invalidMaxTagParams) error {
	return nil
}

type recordingServer struct {
	routes []recordedRoute
}

type recordedRoute struct {
	method string
	path   string
}

func (s *recordingServer) Start(string) error {
	return nil
}

func (s *recordingServer) Stop(context.Context) error {
	return nil
}

func (s *recordingServer) RegisterRoute(method, path string, _ web.HandlerFunc) error {
	s.routes = append(s.routes, recordedRoute{method: method, path: path})
	return nil
}

func (s *recordingServer) ServeHTTP(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}
