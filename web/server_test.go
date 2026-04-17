package web_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	helix "github.com/enokdev/helix"
	"github.com/enokdev/helix/web"
)

func TestServer_RegisterRouteHandlesRequest(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	var gotID string

	if err := server.RegisterRoute(http.MethodGet, "/hello/:id", func(ctx web.Context) error {
		gotID = ctx.Param("id")
		return nil
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/hello/42", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if gotID != "42" {
		t.Fatalf("ctx.Param(%q) = %q, want %q", "id", gotID, "42")
	}
}

func TestContextExposesParamHeaderAndIP(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	var gotParam, gotHeader, gotIP string

	if err := server.RegisterRoute(http.MethodGet, "/users/:id", func(ctx web.Context) error {
		gotParam = ctx.Param("id")
		gotHeader = ctx.Header("X-Test")
		gotIP = ctx.IP()
		return nil
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req.Header.Set("X-Test", "helix")
	req.RemoteAddr = "203.0.113.9:12345"

	resp, err := server.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if gotParam != "123" {
		t.Fatalf("Param() = %q, want %q", gotParam, "123")
	}
	if gotHeader != "helix" {
		t.Fatalf("Header() = %q, want %q", gotHeader, "helix")
	}
	if gotIP == "" {
		t.Fatal("IP() should not be empty")
	}
}

func TestServer_RegisterRouteSupportsHTTPMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
	}{
		{name: "get", method: http.MethodGet},
		{name: "post", method: http.MethodPost},
		{name: "put", method: http.MethodPut},
		{name: "patch", method: http.MethodPatch},
		{name: "delete", method: http.MethodDelete},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t)
			called := false

			if err := server.RegisterRoute(tt.method, "/resource", func(web.Context) error {
				called = true
				return nil
			}); err != nil {
				t.Fatalf("RegisterRoute() error = %v", err)
			}

			resp, err := server.ServeHTTP(httptest.NewRequest(tt.method, "/resource", nil))
			if err != nil {
				t.Fatalf("ServeHTTP() error = %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if !called {
				t.Fatal("handler was not called")
			}
		})
	}
}

func TestServer_RegisterRouteRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		method  string
		path    string
		handler web.HandlerFunc
	}{
		{name: "empty method", method: "", path: "/ok", handler: func(web.Context) error { return nil }},
		{name: "unsupported method", method: http.MethodTrace, path: "/ok", handler: func(web.Context) error { return nil }},
		{name: "empty path", method: http.MethodGet, path: "", handler: func(web.Context) error { return nil }},
		{name: "path without slash", method: http.MethodGet, path: "ok", handler: func(web.Context) error { return nil }},
		{name: "path with trailing space", method: http.MethodGet, path: "/ok ", handler: func(web.Context) error { return nil }},
		{name: "path with embedded space", method: http.MethodGet, path: "/ok path", handler: func(web.Context) error { return nil }},
		{name: "nil handler", method: http.MethodGet, path: "/ok", handler: nil},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newTestServer(t).RegisterRoute(tt.method, tt.path, tt.handler)
			if !errors.Is(err, web.ErrInvalidRoute) {
				t.Fatalf("RegisterRoute() error = %v, want ErrInvalidRoute", err)
			}
		})
	}
}

func TestServer_StopWithoutStart(t *testing.T) {
	t.Parallel()

	if err := newTestServer(t).Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestServer_StopRejectsNilContext(t *testing.T) {
	t.Parallel()

	if err := newTestServer(t).Stop(nil); err == nil { //nolint:staticcheck
		t.Fatal("Stop(nil) should return an error")
	}
}

func TestServer_ServeHTTPRejectsNilRequest(t *testing.T) {
	t.Parallel()

	_, err := newTestServer(t).ServeHTTP(nil)
	if err == nil {
		t.Fatal("ServeHTTP(nil) should return an error")
	}
}

func TestRegisterErrorHandlerOverridesDefaultErrorResponse(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	handler := &ValidationErrorHandler{}
	if err := web.RegisterErrorHandler(server, handler); err != nil {
		t.Fatalf("RegisterErrorHandler() error = %v", err)
	}
	if err := server.RegisterRoute(http.MethodGet, "/users/:id", func(web.Context) error {
		return helix.ValidationError{Message: "email is required", Field: "email"}
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/users/42", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	detail := assertErrorResponse(t, resp, http.StatusUnprocessableEntity, "CUSTOM_VALIDATION", "email")
	if detail.Type != "CustomValidationError" {
		t.Fatalf("error.type = %q, want CustomValidationError", detail.Type)
	}
	if !handler.called {
		t.Fatal("central error handler was not called")
	}
}

func TestRegisterErrorHandlerSupportsContextAndMultipleHandledTypes(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	handler := &ApplicationErrorHandler{}
	if err := web.RegisterErrorHandler(server, handler); err != nil {
		t.Fatalf("RegisterErrorHandler() error = %v", err)
	}

	if err := server.RegisterRoute(http.MethodGet, "/validation/:id", func(web.Context) error {
		return helix.ValidationError{Message: "bad user", Field: "id"}
	}); err != nil {
		t.Fatalf("RegisterRoute(validation) error = %v", err)
	}
	if err := server.RegisterRoute(http.MethodGet, "/missing/:id", func(web.Context) error {
		return helix.NotFoundError{Message: "user missing"}
	}); err != nil {
		t.Fatalf("RegisterRoute(missing) error = %v", err)
	}

	validationResp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/validation/42", nil))
	if err != nil {
		t.Fatalf("ServeHTTP(validation) error = %v", err)
	}
	defer validationResp.Body.Close()
	validationDetail := assertErrorResponse(t, validationResp, http.StatusUnprocessableEntity, "APP_VALIDATION", "id")
	if validationDetail.Message != "validation:bad user" {
		t.Fatalf("validation message = %q, want validation:bad user", validationDetail.Message)
	}

	missingResp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/missing/abc", nil))
	if err != nil {
		t.Fatalf("ServeHTTP(missing) error = %v", err)
	}
	defer missingResp.Body.Close()
	missingDetail := assertErrorResponse(t, missingResp, http.StatusGone, "APP_NOT_FOUND", "")
	if missingDetail.Message != "missing:abc:user missing" {
		t.Fatalf("missing message = %q, want missing:abc:user missing", missingDetail.Message)
	}
}

func TestRegisterErrorHandlerSupportsMultipleHandlersAndWrappedErrors(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	if err := web.RegisterErrorHandler(server, &ValidationErrorHandler{}); err != nil {
		t.Fatalf("RegisterErrorHandler(validation) error = %v", err)
	}
	if err := web.RegisterErrorHandler(server, &NotFoundErrorHandler{}); err != nil {
		t.Fatalf("RegisterErrorHandler(not found) error = %v", err)
	}

	if err := server.RegisterRoute(http.MethodGet, "/wrapped-validation", func(web.Context) error {
		return fmt.Errorf("service: validate user: %w", helix.ValidationError{Message: "email invalid", Field: "email"})
	}); err != nil {
		t.Fatalf("RegisterRoute(validation) error = %v", err)
	}
	if err := server.RegisterRoute(http.MethodGet, "/not-found", func(web.Context) error {
		return helix.NotFoundError{Message: "user missing"}
	}); err != nil {
		t.Fatalf("RegisterRoute(not found) error = %v", err)
	}

	validationResp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/wrapped-validation", nil))
	if err != nil {
		t.Fatalf("ServeHTTP(validation) error = %v", err)
	}
	defer validationResp.Body.Close()
	assertErrorResponse(t, validationResp, http.StatusUnprocessableEntity, "CUSTOM_VALIDATION", "email")

	notFoundResp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/not-found", nil))
	if err != nil {
		t.Fatalf("ServeHTTP(not found) error = %v", err)
	}
	defer notFoundResp.Body.Close()
	assertErrorResponse(t, notFoundResp, http.StatusGone, "CUSTOM_NOT_FOUND", "")
}

func TestRegisterErrorHandlerFallsBackWhenNoHandlerMatches(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	if err := web.RegisterErrorHandler(server, &ValidationErrorHandler{}); err != nil {
		t.Fatalf("RegisterErrorHandler() error = %v", err)
	}
	if err := server.RegisterRoute(http.MethodGet, "/generic", func(web.Context) error {
		return errors.New("database down")
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/generic", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	assertErrorResponse(t, resp, http.StatusInternalServerError, "INTERNAL_ERROR", "")
}

func TestRegisterErrorHandlerRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler any
	}{
		{name: "nil", handler: nil},
		{name: "non pointer", handler: ValidationErrorHandler{}},
		{name: "nil pointer", handler: (*ValidationErrorHandler)(nil)},
		{name: "missing marker", handler: &UnmarkedErrorHandler{}},
		{name: "missing suffix", handler: &NoSuffix{}},
		{name: "malformed directive", handler: &MalformedDirectiveErrorHandler{}},
		{name: "missing directive", handler: &MissingDirectiveErrorHandler{}},
		{name: "invalid signature", handler: &InvalidSignatureErrorHandler{}},
		{name: "unknown handled type", handler: &UnknownTypeErrorHandler{}},
		{name: "duplicate type same handler", handler: &DuplicateValidationErrorHandler{}},
		{name: "interface error argument", handler: &InterfaceArgErrorHandler{}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := web.RegisterErrorHandler(newTestServer(t), tt.handler)
			if !errors.Is(err, web.ErrInvalidErrorHandler) {
				t.Fatalf("RegisterErrorHandler() error = %v, want ErrInvalidErrorHandler", err)
			}
		})
	}
}

func TestRegisterErrorHandlerRejectsDuplicateTypeAcrossHandlers(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	if err := web.RegisterErrorHandler(server, &ValidationErrorHandler{}); err != nil {
		t.Fatalf("RegisterErrorHandler(first) error = %v", err)
	}
	err := web.RegisterErrorHandler(server, &SecondValidationErrorHandler{})
	if !errors.Is(err, web.ErrInvalidErrorHandler) {
		t.Fatalf("RegisterErrorHandler(second) error = %v, want ErrInvalidErrorHandler", err)
	}
}

func TestNoPublicFiberImports(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	allowedDir := filepath.Join(root, "web", "internal")
	fiberImport := strings.Join([]string{"github.com", "gofiber", "fiber", "v2"}, "/")

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(content), fiberImport) {
			return nil
		}
		if !strings.HasPrefix(path, allowedDir+string(filepath.Separator)) && path != allowedDir {
			t.Errorf("Fiber import found in %s; only files under %s may import it", path, allowedDir)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
}

type ValidationErrorHandler struct {
	helix.ErrorHandler
	called bool
}

//helix:handles ValidationError
func (h *ValidationErrorHandler) Handle(err helix.ValidationError) (any, int) {
	h.called = true
	return web.ErrorResponse{Error: web.ErrorDetail{
		Type:    "CustomValidationError",
		Message: "custom:" + err.Error(),
		Field:   err.ErrorField(),
		Code:    "CUSTOM_VALIDATION",
	}}, http.StatusUnprocessableEntity
}

type NotFoundErrorHandler struct {
	helix.ErrorHandler
}

//helix:handles NotFoundError
func (h *NotFoundErrorHandler) Handle(err helix.NotFoundError) (any, int) {
	return web.ErrorResponse{Error: web.ErrorDetail{
		Type:    "CustomNotFoundError",
		Message: "custom:" + err.Error(),
		Code:    "CUSTOM_NOT_FOUND",
	}}, http.StatusGone
}

type ApplicationErrorHandler struct {
	helix.ErrorHandler
}

//helix:handles ValidationError
func (h *ApplicationErrorHandler) Validation(err helix.ValidationError) (any, int) {
	return web.ErrorResponse{Error: web.ErrorDetail{
		Type:    "AppValidationError",
		Message: "validation:" + err.Error(),
		Field:   err.ErrorField(),
		Code:    "APP_VALIDATION",
	}}, http.StatusUnprocessableEntity
}

//helix:handles NotFoundError
func (h *ApplicationErrorHandler) NotFound(ctx web.Context, err helix.NotFoundError) (any, int) {
	return web.ErrorResponse{Error: web.ErrorDetail{
		Type:    "AppNotFoundError",
		Message: "missing:" + ctx.Param("id") + ":" + err.Error(),
		Code:    "APP_NOT_FOUND",
	}}, http.StatusGone
}

type UnmarkedErrorHandler struct{}

//helix:handles ValidationError
func (h *UnmarkedErrorHandler) Handle(_ helix.ValidationError) (any, int) {
	return nil, http.StatusBadRequest
}

type NoSuffix struct {
	helix.ErrorHandler
}

//helix:handles ValidationError
func (h *NoSuffix) Handle(_ helix.ValidationError) (any, int) {
	return nil, http.StatusBadRequest
}

type MalformedDirectiveErrorHandler struct {
	helix.ErrorHandler
}

// helix:handles ValidationError
func (h *MalformedDirectiveErrorHandler) Handle(_ helix.ValidationError) (any, int) {
	return nil, http.StatusBadRequest
}

type MissingDirectiveErrorHandler struct {
	helix.ErrorHandler
}

func (h *MissingDirectiveErrorHandler) Handle(_ helix.ValidationError) (any, int) {
	return nil, http.StatusBadRequest
}

type InvalidSignatureErrorHandler struct {
	helix.ErrorHandler
}

//helix:handles ValidationError
func (h *InvalidSignatureErrorHandler) Handle(_ string) (any, int) {
	return nil, http.StatusBadRequest
}

type UnknownTypeErrorHandler struct {
	helix.ErrorHandler
}

//helix:handles MissingError
func (h *UnknownTypeErrorHandler) Handle(_ helix.ValidationError) (any, int) {
	return nil, http.StatusBadRequest
}

type DuplicateValidationErrorHandler struct {
	helix.ErrorHandler
}

//helix:handles ValidationError
func (h *DuplicateValidationErrorHandler) First(_ helix.ValidationError) (any, int) {
	return nil, http.StatusBadRequest
}

//helix:handles ValidationError
func (h *DuplicateValidationErrorHandler) Second(_ helix.ValidationError) (any, int) {
	return nil, http.StatusBadRequest
}

type SecondValidationErrorHandler struct {
	helix.ErrorHandler
}

//helix:handles ValidationError
func (h *SecondValidationErrorHandler) Handle(_ helix.ValidationError) (any, int) {
	return nil, http.StatusBadRequest
}

type InterfaceArgErrorHandler struct {
	helix.ErrorHandler
}

//helix:handles ValidationError
func (h *InterfaceArgErrorHandler) Handle(_ error) (any, int) {
	return nil, http.StatusBadRequest
}

func newTestServer(t *testing.T) web.HTTPServer {
	t.Helper()
	return web.NewServer()
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("moduleRoot: go.mod not found")
		}
		dir = parent
	}
}
