package web_test

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	var gotMethod, gotURL, gotParam, gotHeader, gotIP string

	if err := server.RegisterRoute(http.MethodGet, "/users/:id", func(ctx web.Context) error {
		gotMethod = ctx.Method()
		gotURL = ctx.OriginalURL()
		gotParam = ctx.Param("id")
		gotHeader = ctx.Header("X-Test")
		gotIP = ctx.IP()
		return nil
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/users/123?active=true", nil)
	req.Header.Set("X-Test", "helix")
	req.RemoteAddr = "203.0.113.9:12345"

	resp, err := server.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if gotMethod != http.MethodGet {
		t.Fatalf("Method() = %q, want %q", gotMethod, http.MethodGet)
	}
	if gotURL != "/users/123?active=true" {
		t.Fatalf("OriginalURL() = %q, want %q", gotURL, "/users/123?active=true")
	}
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

// ---------- RouteObserver tests ----------

type testObserver struct {
	calls []web.RouteObservation
}

func (o *testObserver) Observe(obs web.RouteObservation) {
	o.calls = append(o.calls, obs)
}

func TestWithRouteObserver_RecitObservation(t *testing.T) {
	t.Parallel()

	obs := &testObserver{}
	server := web.NewServer(web.WithRouteObserver(obs))

	if err := server.RegisterRoute(http.MethodGet, "/ping", func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"msg": "pong"})
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/ping", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if len(obs.calls) != 1 {
		t.Fatalf("observer received %d calls, want 1", len(obs.calls))
	}
	got := obs.calls[0]
	if got.Method != http.MethodGet {
		t.Errorf("Method = %q, want GET", got.Method)
	}
	if got.Route != "/ping" {
		t.Errorf("Route = %q, want /ping", got.Route)
	}
	if got.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", got.StatusCode)
	}
	if got.Duration <= 0 {
		t.Error("Duration should be positive")
	}
}

func TestWithRouteObserver_CaptureRouteTemplate(t *testing.T) {
	t.Parallel()

	obs := &testObserver{}
	server := web.NewServer(web.WithRouteObserver(obs))

	if err := server.RegisterRoute(http.MethodGet, "/users/:id", func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(nil)
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}

	for _, id := range []string{"1", "2"} {
		resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
		if err != nil {
			t.Fatalf("ServeHTTP() error = %v", err)
		}
		resp.Body.Close()
	}

	if len(obs.calls) != 2 {
		t.Fatalf("observer received %d calls, want 2", len(obs.calls))
	}
	for i, call := range obs.calls {
		if call.Route != "/users/:id" {
			t.Errorf("call[%d].Route = %q, want /users/:id", i, call.Route)
		}
	}
}

func TestWithRouteObserver_CaptureStatusErreurStructuree(t *testing.T) {
	t.Parallel()

	obs := &testObserver{}
	server := web.NewServer(web.WithRouteObserver(obs))

	if err := server.RegisterRoute(http.MethodGet, "/err", func(_ web.Context) error {
		return web.Forbidden("nope")
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/err", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if len(obs.calls) != 1 {
		t.Fatalf("observer received %d calls, want 1", len(obs.calls))
	}
	if obs.calls[0].StatusCode != http.StatusForbidden {
		t.Errorf("StatusCode = %d, want 403", obs.calls[0].StatusCode)
	}
}

func TestWithRouteObserver_CaptureStatusErreurGenerique(t *testing.T) {
	t.Parallel()

	obs := &testObserver{}
	server := web.NewServer(web.WithRouteObserver(obs))

	if err := server.RegisterRoute(http.MethodGet, "/boom", func(_ web.Context) error {
		return errors.New("database down")
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/boom", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if len(obs.calls) != 1 {
		t.Fatalf("observer received %d calls, want 1", len(obs.calls))
	}
	if obs.calls[0].StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", obs.calls[0].StatusCode)
	}
}

// ---------- SetHeader / Send tests ----------

func TestContext_SetHeaderEtSend(t *testing.T) {
	t.Parallel()

	server := web.NewServer()
	if err := server.RegisterRoute(http.MethodGet, "/raw", func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		ctx.SetHeader("Content-Type", "text/plain; charset=utf-8")
		return ctx.Send([]byte("hello helix"))
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/raw", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain...", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello helix" {
		t.Errorf("body = %q, want 'hello helix'", body)
	}
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

// ---- Task 1: Panic Recovery ----

func TestServer_PanicRecovery(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	if err := server.RegisterRoute(http.MethodGet, "/panic", func(_ web.Context) error {
		panic("unexpected nil pointer")
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}
	if err := server.RegisterRoute(http.MethodGet, "/healthy", func(ctx web.Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"status": "ok"})
	}); err != nil {
		t.Fatalf("RegisterRoute() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/panic", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()
	assertErrorResponse(t, resp, http.StatusInternalServerError, "INTERNAL_ERROR", "")

	// Server must still be alive after a panic.
	resp2, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/healthy", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() second request error = %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second request status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}
}

// ---- Task 3: JSON Serialization Failure ----

func TestServer_JSONSerializationFailure(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	if err := web.RegisterController(server, &ChannelPayloadController{}); err != nil {
		t.Fatalf("RegisterController() error = %v", err)
	}

	resp, err := server.ServeHTTP(httptest.NewRequest(http.MethodGet, "/channel-payloads", nil))
	if err != nil {
		t.Fatalf("ServeHTTP() error = %v", err)
	}
	defer resp.Body.Close()
	assertErrorResponse(t, resp, http.StatusInternalServerError, "INTERNAL_ERROR", "")
}
