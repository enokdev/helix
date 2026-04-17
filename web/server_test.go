package web_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	if err := newTestServer(t).Stop(nil); err == nil {
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
