package web_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
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

func (c *UnsupportedSignatureController) Index(params struct{}) error {
	return nil
}
