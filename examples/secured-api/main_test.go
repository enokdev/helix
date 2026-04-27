package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enokdev/helix/security"
)

func TestLoadConfigReadsExampleYAML(t *testing.T) {
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.Server.Port != 8081 {
		t.Fatalf("server.port = %d, want 8081", cfg.Server.Port)
	}
	if cfg.App.Name != "helix-secured-api" {
		t.Fatalf("app.name = %q, want helix-secured-api", cfg.App.Name)
	}
}

func TestExampleDocumentationExists(t *testing.T) {
	path := "README.md"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = filepath.Join("examples", "secured-api", "README.md")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("README.md must exist: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"go run ./examples/secured-api",
		"go test ./examples/secured-api",
		"POST /auth/login",
		"GET /api/profile",
		"GET /admin/users",
		"Bearer",
		"Authentication",
		"RBAC",
		"config/application.yaml",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("README.md missing %q", want)
		}
	}
}

func TestExampleDoesNotImportFiberDirectly(t *testing.T) {
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	fiberImport := strings.Join([]string{"github.com", "gofiber", "fiber", "v2"}, "/")
	if strings.Contains(string(data), fiberImport) {
		t.Fatal("example must use Helix web abstractions instead of importing Fiber directly")
	}
}

func TestExampleDoesNotImportJWTDirectly(t *testing.T) {
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	jwtImport := strings.Join([]string{"github.com", "golang-jwt", "jwt"}, "/")
	if strings.Contains(string(data), jwtImport) {
		t.Fatal("example must use Helix security abstractions instead of importing golang-jwt directly")
	}
}

func TestLoginSuccess(t *testing.T) {
	jwtSvc, err := newJWTService()
	if err != nil {
		t.Fatalf("newJWTService() error = %v", err)
	}

	server, err := newServer(jwtSvc, 1*time.Hour)
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}

	// Test user login
	loginBody := bytes.NewBufferString(`{"username":"user","password":"password"}`)
	resp := serve(t, server, http.MethodPost, "/auth/login", loginBody)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /auth/login status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}

	if loginResp.Token == "" || loginResp.TokenType != "Bearer" {
		t.Fatalf("invalid login response: %+v", loginResp)
	}
	if loginResp.Username != "user" || loginResp.Role != "user" {
		t.Fatalf("user mismatch: %+v", loginResp)
	}

	// Test admin login
	adminLoginBody := bytes.NewBufferString(`{"username":"admin","password":"password"}`)
	adminResp := serve(t, server, http.MethodPost, "/auth/login", adminLoginBody)
	if adminResp.StatusCode != http.StatusOK {
		t.Fatalf("POST /auth/login for admin status = %d, want %d", adminResp.StatusCode, http.StatusOK)
	}

	var adminLoginResp LoginResponse
	if err := json.NewDecoder(adminResp.Body).Decode(&adminLoginResp); err != nil {
		t.Fatalf("decode admin login response: %v", err)
	}

	if adminLoginResp.Username != "admin" {
		t.Fatalf("admin login failed: %+v", adminLoginResp)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	jwtSvc, err := newJWTService()
	if err != nil {
		t.Fatalf("newJWTService() error = %v", err)
	}

	server, err := newServer(jwtSvc, 1*time.Hour)
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}

	// Test invalid credentials
	loginBody := bytes.NewBufferString(`{"username":"invalid","password":"wrong"}`)
	resp := serve(t, server, http.MethodPost, "/auth/login", loginBody)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST /auth/login with invalid creds status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestProtectedEndpointWithoutToken(t *testing.T) {
	jwtSvc, err := newJWTService()
	if err != nil {
		t.Fatalf("newJWTService() error = %v", err)
	}

	server, err := newServer(jwtSvc, 1*time.Hour)
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}

	// Test accessing /api/profile without token
	resp := serve(t, server, http.MethodGet, "/api/profile", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/profile without token status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	// Test accessing /admin/users without token
	respAdmin := serve(t, server, http.MethodGet, "/admin/users", nil)
	if respAdmin.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /admin/users without token status = %d, want %d", respAdmin.StatusCode, http.StatusUnauthorized)
	}
}

func TestProtectedEndpointWithValidToken(t *testing.T) {
	jwtSvc, err := newJWTService()
	if err != nil {
		t.Fatalf("newJWTService() error = %v", err)
	}

	server, err := newServer(jwtSvc, 1*time.Hour)
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}

	// Get a valid token
	token, err := jwtSvc.Generate(map[string]any{
		"username": "user",
		"roles":    []string{"user"},
	})
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Test accessing /api/profile with valid token
	resp := serveWithAuth(t, server, http.MethodGet, "/api/profile", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/profile with valid token status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var profileResp AccountInfo
	if err := json.NewDecoder(resp.Body).Decode(&profileResp); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}

	if profileResp.Username != "user" {
		t.Fatalf("profile username = %q, want user", profileResp.Username)
	}
}

func TestAdminEndpointWithUserRole(t *testing.T) {
	jwtSvc, err := newJWTService()
	if err != nil {
		t.Fatalf("newJWTService() error = %v", err)
	}

	server, err := newServer(jwtSvc, 1*time.Hour)
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}

	// Get a token with user role only
	token, err := jwtSvc.Generate(map[string]any{
		"username": "user",
		"roles":    []string{"user"},
	})
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Test accessing /admin/users with user role (should be forbidden)
	resp := serveWithAuth(t, server, http.MethodGet, "/admin/users", nil, token)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("GET /admin/users with user role status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestAdminEndpointWithAdminRole(t *testing.T) {
	jwtSvc, err := newJWTService()
	if err != nil {
		t.Fatalf("newJWTService() error = %v", err)
	}

	server, err := newServer(jwtSvc, 1*time.Hour)
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}

	// Get a token with admin role
	token, err := jwtSvc.Generate(map[string]any{
		"username": "admin",
		"roles":    []string{"admin", "user"},
	})
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Test accessing /admin/users with admin role (should succeed)
	resp := serveWithAuth(t, server, http.MethodGet, "/admin/users", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /admin/users with admin role status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var userList UserList
	if err := json.NewDecoder(resp.Body).Decode(&userList); err != nil {
		t.Fatalf("decode user list: %v", err)
	}

	if len(userList.Users) == 0 {
		t.Fatal("expected at least one user in list")
	}
}

func newJWTService() (*security.JWTService, error) {
	return security.NewJWTService("test-secret-key-for-testing", 1*time.Hour)
}

func serve(t *testing.T, server interface {
	ServeHTTP(*http.Request) (*http.Response, error)
}, method, path string, body *bytes.Buffer) *http.Response {
	t.Helper()
	if body == nil {
		body = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, body)
	if body.Len() > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := server.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP(%s %s) error = %v", method, path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func serveWithAuth(t *testing.T, server interface {
	ServeHTTP(*http.Request) (*http.Response, error)
}, method, path string, body *bytes.Buffer, token string) *http.Response {
	t.Helper()
	if body == nil {
		body = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, body)
	if body.Len() > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := server.ServeHTTP(req)
	if err != nil {
		t.Fatalf("ServeHTTP(%s %s) error = %v", method, path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}
