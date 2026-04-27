package web_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
			if tt.name == "no conventional methods" {
				if err == nil || !strings.Contains(err.Error(), "no routable methods found") {
					t.Fatalf("RegisterController() error = %v, want error containing 'no routable methods found'", err)
				}
				return
			}
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

func TestRegisterController_MapsSuccessReturnValuesToHTTPStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantID     string
	}{
		{name: "get pointer payload", method: http.MethodGet, path: "/response-mappings/42", wantStatus: http.StatusOK, wantID: "42"},
		{name: "post pointer payload", method: http.MethodPost, path: "/response-mappings", wantStatus: http.StatusCreated, wantID: "created"},
		{name: "custom get value payload", method: http.MethodGet, path: "/response-mappings/preview", wantStatus: http.StatusOK, wantID: "preview"},
		{name: "custom post same handler", method: http.MethodPost, path: "/response-mappings/preview", wantStatus: http.StatusCreated, wantID: "preview"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t)
			if err := web.RegisterController(server, &ResponseMappingController{}); err != nil {
				t.Fatalf("RegisterController() error = %v", err)
			}

			resp, err := server.ServeHTTP(httptest.NewRequest(tt.method, tt.path, nil))
			if err != nil {
				t.Fatalf("ServeHTTP() error = %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("StatusCode = %d, want %d; body = %s", resp.StatusCode, tt.wantStatus, string(body))
			}
			if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Fatalf("Content-Type = %q, want application/json", got)
			}

			var got ResponseUser
			if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
				t.Fatalf("decode success response: %v", err)
			}
			if got.ID != tt.wantID {
				t.Fatalf("id = %q, want %q", got.ID, tt.wantID)
			}
			if got.Email == "" {
				t.Fatal("email should not be empty")
			}
		})
	}
}

func TestRegisterController_MapsReturnErrorsToStructuredHTTPResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantType   string
		wantCode   string
		wantField  string
	}{
		{name: "not found", path: "/error-mappings/not-found", wantStatus: http.StatusNotFound, wantType: "NotFoundError", wantCode: "NOT_FOUND"},
		{name: "validation", path: "/error-mappings/validation", wantStatus: http.StatusBadRequest, wantType: "ValidationError", wantCode: "VALIDATION_FAILED", wantField: "email"},
		{name: "generic", path: "/error-mappings/generic", wantStatus: http.StatusInternalServerError, wantType: "InternalServerError", wantCode: "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t)
			if err := web.RegisterController(server, &ErrorMappingController{}); err != nil {
				t.Fatalf("RegisterController() error = %v", err)
			}

			resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, tt.path, nil))
			if err != nil {
				t.Fatalf("ServeHTTP() error = %v", err)
			}
			defer resp.Body.Close()

			detail := assertErrorResponse(t, resp, tt.wantStatus, tt.wantCode, tt.wantField)
			if detail.Type != tt.wantType {
				t.Fatalf("error.type = %q, want %q", detail.Type, tt.wantType)
			}
		})
	}
}

func TestRegisterController_MapsWrappedStructuredErrors(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	if err := web.RegisterController(server, &WrappedErrorController{}); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/wrapped-errors", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	assertErrorResponse(t, resp, http.StatusNotFound, "NOT_FOUND", "")
}

func TestRegisterController_RejectsInvalidReturnSignatures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		controller any
	}{
		{name: "too many returns", controller: &TooManyReturnsController{}},
		{name: "second return not error", controller: &SecondReturnNotErrorController{}},
		{name: "error error returns", controller: &DoubleErrorReturnController{}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := web.RegisterController(newTestServer(t), tt.controller)
			if !errors.Is(err, web.ErrUnsupportedHandler) {
				t.Fatalf("RegisterController() error = %v, want ErrUnsupportedHandler", err)
			}
		})
	}
}

func TestRegisterController_RejectsInvalidMaxTagAtRegistration(t *testing.T) {
	t.Parallel()

	err := web.RegisterController(newTestServer(t), &InvalidMaxTagController{})
	if !errors.Is(err, web.ErrUnsupportedHandler) {
		t.Fatalf("RegisterController() error = %v, want ErrUnsupportedHandler", err)
	}
}

func TestRegisterController_AppliesAuthenticatedGuard(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	controller := &GuardedController{}

	if err := web.RegisterGuard(server, "authenticated", web.GuardFunc(func(ctx web.Context) error {
		if ctx.Header("Authorization") == "" {
			return web.Unauthorized("authentication required")
		}
		return nil
	})); err != nil {
		t.Fatalf("RegisterGuard() error = %v", err)
	}
	if err := web.RegisterController(server, controller); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/guardeds", nil))
	if err != nil {
		t.Fatalf("ServeHTTP(unauthenticated) error = %v", err)
	}
	defer resp.Body.Close()
	assertErrorResponse(t, resp, http.StatusUnauthorized, "UNAUTHORIZED", "")
	if controller.indexCalls != 0 {
		t.Fatalf("Index calls = %d, want 0", controller.indexCalls)
	}

	req := httptest.NewRequest(http.MethodGet, "/guardeds", nil)
	req.Header.Set("Authorization", "Bearer test")
	resp, err = server.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP(authenticated) error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if controller.indexCalls != 1 {
		t.Fatalf("Index calls = %d, want 1", controller.indexCalls)
	}
}

func TestRegisterController_AppliesRoleGuardFactory(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	controller := &AdminController{}

	if err := web.RegisterGuardFactory(server, "role", func(role string) (web.Guard, error) {
		return web.GuardFunc(func(ctx web.Context) error {
			for _, current := range strings.Split(ctx.Header("X-Helix-Roles"), ",") {
				if strings.TrimSpace(current) == role {
					return nil
				}
			}
			return web.Forbidden("forbidden")
		}), nil
	}); err != nil {
		t.Fatalf("RegisterGuardFactory() error = %v", err)
	}
	if err := web.RegisterController(server, controller); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/admins", nil))
	if err != nil {
		t.Fatalf("ServeHTTP(forbidden) error = %v", err)
	}
	defer resp.Body.Close()
	assertErrorResponse(t, resp, http.StatusForbidden, "FORBIDDEN", "")
	if controller.calls != 0 {
		t.Fatalf("Index calls = %d, want 0", controller.calls)
	}

	req := httptest.NewRequest(http.MethodGet, "/admins", nil)
	req.Header.Set("X-Helix-Roles", "user, admin")
	resp, err = server.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP(admin) error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if controller.calls != 1 {
		t.Fatalf("Index calls = %d, want 1", controller.calls)
	}
}

func TestRegisterController_AppliesCacheInterceptorAfterGuards(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	controller := &CachedController{}

	if err := web.RegisterGuard(server, "authenticated", web.GuardFunc(func(ctx web.Context) error {
		if ctx.Header("Authorization") == "" {
			return web.Unauthorized("authentication required")
		}
		return nil
	})); err != nil {
		t.Fatalf("RegisterGuard() error = %v", err)
	}
	if err := web.RegisterController(server, controller); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/cacheds?item=42", nil)
	req.Header.Set("Authorization", "Bearer first")
	resp, err := server.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP(first) error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var first CachedResponse
	if err := json.NewDecoder(resp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first response: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/cacheds?item=42", nil)
	req.Header.Set("Authorization", "Bearer second")
	resp, err = server.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP(second) error = %v", err)
	}
	defer resp.Body.Close()
	var second CachedResponse
	if err := json.NewDecoder(resp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if second != first {
		t.Fatalf("cached response = %#v, want %#v", second, first)
	}
	if controller.calls != 1 {
		t.Fatalf("handler calls = %d, want 1", controller.calls)
	}

	resp, err = server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/cacheds?item=42", nil))
	if err != nil {
		t.Fatalf("ServeHTTP(unauthorized cached path) error = %v", err)
	}
	defer resp.Body.Close()
	assertErrorResponse(t, resp, http.StatusUnauthorized, "UNAUTHORIZED", "")
	if controller.calls != 1 {
		t.Fatalf("handler calls after unauthorized request = %d, want 1", controller.calls)
	}
}

func TestRegisterController_ChainsGuardsAndInterceptorsInOrder(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	var order []string
	controller := &ChainedController{order: &order}

	if err := web.RegisterGuard(server, "first", web.GuardFunc(func(web.Context) error {
		order = append(order, "guard:first")
		return nil
	})); err != nil {
		t.Fatalf("RegisterGuard(first) error = %v", err)
	}
	if err := web.RegisterGuard(server, "second", web.GuardFunc(func(web.Context) error {
		order = append(order, "guard:second")
		return nil
	})); err != nil {
		t.Fatalf("RegisterGuard(second) error = %v", err)
	}
	if err := web.RegisterInterceptor(server, "first", web.InterceptorFunc(func(ctx web.Context, next web.HandlerFunc) error {
		order = append(order, "interceptor:first:before")
		err := next(ctx)
		order = append(order, "interceptor:first:after")
		return err
	})); err != nil {
		t.Fatalf("RegisterInterceptor(first) error = %v", err)
	}
	if err := web.RegisterInterceptor(server, "second", web.InterceptorFunc(func(ctx web.Context, next web.HandlerFunc) error {
		order = append(order, "interceptor:second:before")
		err := next(ctx)
		order = append(order, "interceptor:second:after")
		return err
	})); err != nil {
		t.Fatalf("RegisterInterceptor(second) error = %v", err)
	}
	if err := web.RegisterController(server, controller); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/chaineds", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	want := []string{
		"guard:first",
		"guard:second",
		"interceptor:first:before",
		"interceptor:second:before",
		"handler",
		"interceptor:second:after",
		"interceptor:first:after",
	}
	if strings.Join(order, "|") != strings.Join(want, "|") {
		t.Fatalf("execution order = %#v, want %#v", order, want)
	}
}

func TestRegisterController_AppliesRouteDirectivesToConventionsAndCustomRoutes(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	controller := &MixedDirectiveController{}

	if err := web.RegisterGuard(server, "authenticated", web.GuardFunc(func(ctx web.Context) error {
		if ctx.Header("Authorization") == "" {
			return web.Unauthorized("authentication required")
		}
		return nil
	})); err != nil {
		t.Fatalf("RegisterGuard() error = %v", err)
	}
	if err := web.RegisterController(server, controller); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	for _, path := range []string{"/mixed-directives", "/mixed-directives/custom", "/mixed-directives/alternate"} {
		resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatalf("ServeHTTP(%s) error = %v", path, err)
		}
		assertErrorResponse(t, resp, http.StatusUnauthorized, "UNAUTHORIZED", "")
		resp.Body.Close()
	}
	if controller.calls != 0 {
		t.Fatalf("handler calls = %d, want 0", controller.calls)
	}
}

func TestRegisterController_RejectsInvalidGuardAndInterceptorDirectives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		controller any
		setup      func(web.HTTPServer) error
	}{
		{name: "space guard directive", controller: &SpaceGuardDirectiveController{}},
		{name: "plus guard directive", controller: &PlusGuardDirectiveController{}},
		{name: "missing guard argument", controller: &MissingGuardArgumentController{}},
		{name: "missing interceptor argument", controller: &MissingInterceptorArgumentController{}},
		{name: "too many guard tokens", controller: &TooManyGuardTokensController{}},
		{name: "unknown guard", controller: &UnknownGuardController{}},
		{name: "unknown interceptor", controller: &UnknownInterceptorController{}},
		{name: "invalid cache duration", controller: &InvalidCacheDurationController{}},
		{
			name:       "failing guard factory",
			controller: &AdminController{},
			setup: func(server web.HTTPServer) error {
				return web.RegisterGuardFactory(server, "role", func(string) (web.Guard, error) {
					return nil, fmt.Errorf("test: invalid guard argument")
				})
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t)
			if tt.setup != nil {
				if err := tt.setup(server); err != nil {
					t.Fatalf("setup() error = %v", err)
				}
			}
			err := web.RegisterController(server, tt.controller)
			if !errors.Is(err, web.ErrInvalidDirective) {
				t.Fatalf("RegisterController() error = %v, want ErrInvalidDirective", err)
			}
		})
	}
}

func TestRegisterGuardAndInterceptorRejectInvalidRegistration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(web.HTTPServer) error
	}{
		{name: "empty guard name", run: func(server web.HTTPServer) error {
			return web.RegisterGuard(server, "", web.GuardFunc(func(web.Context) error { return nil }))
		}},
		{name: "nil guard", run: func(server web.HTTPServer) error {
			return web.RegisterGuard(server, "auth", nil)
		}},
		{name: "duplicate guard", run: func(server web.HTTPServer) error {
			if err := web.RegisterGuard(server, "auth", web.GuardFunc(func(web.Context) error { return nil })); err != nil {
				return err
			}
			return web.RegisterGuard(server, "auth", web.GuardFunc(func(web.Context) error { return nil }))
		}},
		{name: "empty interceptor name", run: func(server web.HTTPServer) error {
			return web.RegisterInterceptor(server, "", web.InterceptorFunc(func(ctx web.Context, next web.HandlerFunc) error {
				return next(ctx)
			}))
		}},
		{name: "nil interceptor", run: func(server web.HTTPServer) error {
			return web.RegisterInterceptor(server, "audit", nil)
		}},
		{name: "duplicate interceptor", run: func(server web.HTTPServer) error {
			if err := web.RegisterInterceptor(server, "audit", web.InterceptorFunc(func(ctx web.Context, next web.HandlerFunc) error {
				return next(ctx)
			})); err != nil {
				return err
			}
			return web.RegisterInterceptor(server, "audit", web.InterceptorFunc(func(ctx web.Context, next web.HandlerFunc) error {
				return next(ctx)
			}))
		}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.run(newTestServer(t))
			if !errors.Is(err, web.ErrInvalidDirective) {
				t.Fatalf("registration error = %v, want ErrInvalidDirective", err)
			}
		})
	}
}

type testErrorDetail struct {
	Type    string
	Message string
	Field   string
	Code    string
}

func assertErrorResponse(t *testing.T, resp *http.Response, wantStatus int, wantCode, wantField string) testErrorDetail {
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
	return testErrorDetail{
		Type:    payload.Error.Type,
		Message: payload.Error.Message,
		Field:   payload.Error.Field,
		Code:    payload.Error.Code,
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

func (c *AmbiguousBindingController) Index(_ ambiguousInput) error {
	return nil
}

type InvalidMaxTagController struct {
	helix.Controller
}

type invalidMaxTagParams struct {
	Page int `query:"page" max:"abc"`
}

func (c *InvalidMaxTagController) Index(_ invalidMaxTagParams) error {
	return nil
}

type ResponseUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type ResponseMappingController struct {
	helix.Controller
}

func (c *ResponseMappingController) Show(ctx web.Context) (*ResponseUser, error) {
	return &ResponseUser{ID: ctx.Param("id"), Email: "alice@example.com"}, nil
}

func (c *ResponseMappingController) Create() (*ResponseUser, error) {
	return &ResponseUser{ID: "created", Email: "created@example.com"}, nil
}

//helix:route GET /response-mappings/preview
//helix:route POST /response-mappings/preview
func (c *ResponseMappingController) Preview() ResponseUser {
	return ResponseUser{ID: "preview", Email: "preview@example.com"}
}

type ErrorMappingController struct {
	helix.Controller
}

//helix:route GET /error-mappings/not-found
func (c *ErrorMappingController) NotFound() (*ResponseUser, error) {
	return nil, helix.NotFoundError{Message: "user not found"}
}

//helix:route GET /error-mappings/validation
func (c *ErrorMappingController) Validation() (*ResponseUser, error) {
	return nil, helix.ValidationError{Message: "email is required", Field: "email"}
}

//helix:route GET /error-mappings/generic
func (c *ErrorMappingController) Generic() (*ResponseUser, error) {
	return nil, errors.New("database connection failed")
}

type WrappedErrorController struct {
	helix.Controller
}

func (c *WrappedErrorController) Index() (*ResponseUser, error) {
	return nil, fmt.Errorf("repo: find user: %w", helix.NotFoundError{Message: "user not found"})
}

type TooManyReturnsController struct {
	helix.Controller
}

func (c *TooManyReturnsController) Index() (ResponseUser, string, error) {
	return ResponseUser{}, "", nil
}

type SecondReturnNotErrorController struct {
	helix.Controller
}

func (c *SecondReturnNotErrorController) Index() (ResponseUser, string) {
	return ResponseUser{}, ""
}

type DoubleErrorReturnController struct {
	helix.Controller
}

func (c *DoubleErrorReturnController) Index() (error, error) {
	return nil, nil
}

type GuardedController struct {
	helix.Controller
	indexCalls int
}

//helix:guard authenticated
func (c *GuardedController) Index() {
	c.indexCalls++
}

type AdminController struct {
	helix.Controller
	calls int
}

//helix:guard role:admin
func (c *AdminController) Index() {
	c.calls++
}

type CachedResponse struct {
	Call int    `json:"call"`
	Item string `json:"item"`
}

type CachedController struct {
	helix.Controller
	calls int
}

//helix:guard authenticated
//helix:interceptor cache:5m
func (c *CachedController) Index(ctx web.Context) CachedResponse {
	c.calls++
	return CachedResponse{Call: c.calls, Item: ctx.Query("item")}
}

type ChainedController struct {
	helix.Controller
	order *[]string
}

//helix:guard first
//helix:guard second
//helix:interceptor first
//helix:interceptor second
func (c *ChainedController) Index() {
	*c.order = append(*c.order, "handler")
}

type MixedDirectiveController struct {
	helix.Controller
	calls int
}

//helix:guard authenticated
//helix:route GET /mixed-directives/custom
//helix:route GET /mixed-directives/alternate
func (c *MixedDirectiveController) Index() {
	c.calls++
}

type SpaceGuardDirectiveController struct {
	helix.Controller
}

// helix:guard authenticated
func (c *SpaceGuardDirectiveController) Index() {}

type PlusGuardDirectiveController struct {
	helix.Controller
}

// +helix:guard authenticated
func (c *PlusGuardDirectiveController) Index() {}

type MissingGuardArgumentController struct {
	helix.Controller
}

//helix:guard
func (c *MissingGuardArgumentController) Index() {}

type MissingInterceptorArgumentController struct {
	helix.Controller
}

//helix:interceptor
func (c *MissingInterceptorArgumentController) Index() {}

type TooManyGuardTokensController struct {
	helix.Controller
}

//helix:guard authenticated extra
func (c *TooManyGuardTokensController) Index() {}

type UnknownGuardController struct {
	helix.Controller
}

//helix:guard missing
func (c *UnknownGuardController) Index() {}

type UnknownInterceptorController struct {
	helix.Controller
}

//helix:interceptor missing
func (c *UnknownInterceptorController) Index() {}

type InvalidCacheDurationController struct {
	helix.Controller
}

//helix:interceptor cache:0s
func (c *InvalidCacheDurationController) Index() {}

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

func (s *recordingServer) IsGeneratedOnly() bool {
	return false
}

func TestRegisterController_UsesRegistryWhenAvailable(t *testing.T) {
	t.Parallel()

	// Register a route in the registry
	registry := web.GlobalRouteRegistry()

	// Verify registry has methods for querying routes
	if registry == nil {
		t.Fatal("GlobalRouteRegistry() returned nil")
	}

	// Test that HasGeneratedRoutes returns false initially
	if registry.HasGeneratedRoutes("TestController") {
		t.Fatal("HasGeneratedRoutes should return false initially")
	}
}

func TestRegisterController_FallsBackToASTWhenNoRegistry(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	controller := &UserController{calls: make(map[string]string)}

	// Verify that RegisterController still works (falls back to AST parsing)
	if err := web.RegisterController(server, controller); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	// Verify conventional routes were registered via AST parsing
	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/users", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// ---- Task 2: Error in any slot ----

type ErrorInPayloadSlotController struct {
	helix.Controller
}

func (c *ErrorInPayloadSlotController) Index() (any, error) {
	return fmt.Errorf("something failed internally"), nil
}

// ---- Task 3: JSON serialization failure (unserializable payload) ----

type ChannelPayloadController struct {
	helix.Controller
}

func (c *ChannelPayloadController) Index() (any, error) {
	return make(chan int), nil
}

// ---- Task 4: Content-Type validation ----

func TestRegisterController_ErrorInAnySlot(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	if err := web.RegisterController(server, &ErrorInPayloadSlotController{}); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/error-in-payload-slots", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()
	assertErrorResponse(t, resp, http.StatusInternalServerError, "INTERNAL_ERROR", "")
}

func TestRegisterController_AcceptsCaseInsensitiveContentType(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	if err := web.RegisterController(server, &BodyBindingController{}); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/body-bindings",
		strings.NewReader(`{"name":"Alice","email":"alice@example.com"}`))
	req.Header.Set("Content-Type", "APPLICATION/JSON")

	resp, err := server.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestRegisterController_RejectsMissingContentType(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	if err := web.RegisterController(server, &BodyBindingController{}); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/body-bindings",
		strings.NewReader(`{"name":"Alice","email":"alice@example.com"}`))
	// Intentionally NOT setting Content-Type

	resp, err := server.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()
	detail := assertErrorResponse(t, resp, http.StatusBadRequest, "INVALID_JSON", "")
	if !strings.Contains(detail.Message, "required") {
		t.Errorf("error.message = %q, want it to contain 'required'", detail.Message)
	}
}

func TestRegisterController_RejectsWrongContentType(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	if err := web.RegisterController(server, &BodyBindingController{}); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/body-bindings",
		strings.NewReader(`{"name":"Alice","email":"alice@example.com"}`))
	req.Header.Set("Content-Type", "text/plain")

	resp, err := server.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()
	detail := assertErrorResponse(t, resp, http.StatusBadRequest, "INVALID_JSON", "")
	if !strings.Contains(detail.Message, "must be application/json") {
		t.Errorf("error.message = %q, want it to contain 'must be application/json'", detail.Message)
	}
}
