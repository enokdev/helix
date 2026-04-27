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
)

func TestLoadConfigReadsExampleYAML(t *testing.T) {
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("server.port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.App.Name != "helix-crud-api" {
		t.Fatalf("app.name = %q, want helix-crud-api", cfg.App.Name)
	}
}

func TestExampleDocumentationExists(t *testing.T) {
	path := "README.md"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = filepath.Join("examples", "crud-api", "README.md")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("README.md must exist: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"go run ./examples/crud-api",
		"go test ./examples/crud-api",
		"POST /users",
		"GET /users",
		"GET /users/:id",
		"PUT /users/:id",
		"DELETE /users/:id",
		"config/application.yaml",
		"UserRepository",
		"UserService",
		"UserController",
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

func TestUsersCRUD(t *testing.T) {
	server, err := newServer()
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}

	createBody := bytes.NewBufferString(`{"name":"Ada Lovelace","email":"ada@example.com"}`)
	createResp := serve(t, server, http.MethodPost, "/users", createBody)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /users status = %d, want %d", createResp.StatusCode, http.StatusCreated)
	}

	var created User
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created user: %v", err)
	}
	if created.ID != 1 || created.Name != "Ada Lovelace" || created.Email != "ada@example.com" {
		t.Fatalf("created user = %+v", created)
	}

	showResp := serve(t, server, http.MethodGet, "/users/1", nil)
	if showResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /users/1 status = %d, want %d", showResp.StatusCode, http.StatusOK)
	}
	var shown User
	if err := json.NewDecoder(showResp.Body).Decode(&shown); err != nil {
		t.Fatalf("decode shown user: %v", err)
	}
	if shown.ID != 1 || shown.Name != "Ada Lovelace" || shown.Email != "ada@example.com" {
		t.Fatalf("shown user = %+v, want ID=1 Name=Ada Lovelace Email=ada@example.com", shown)
	}

	updateBody := bytes.NewBufferString(`{"name":"Ada Byron","email":"ada.byron@example.com"}`)
	updateResp := serve(t, server, http.MethodPut, "/users/1", updateBody)
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /users/1 status = %d, want %d", updateResp.StatusCode, http.StatusOK)
	}
	var updated User
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated user: %v", err)
	}
	if updated.ID != 1 || updated.Name != "Ada Byron" || updated.Email != "ada.byron@example.com" {
		t.Fatalf("updated user = %+v, want ID=1 Name=Ada Byron Email=ada.byron@example.com", updated)
	}

	deleteResp := serve(t, server, http.MethodDelete, "/users/1", nil)
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /users/1 status = %d, want %d", deleteResp.StatusCode, http.StatusNoContent)
	}

	listResp := serve(t, server, http.MethodGet, "/users", nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /users status = %d, want %d", listResp.StatusCode, http.StatusOK)
	}
	var list []User
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("decode user list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("list after delete = %d users, want 0", len(list))
	}
}

func TestNegativePaths(t *testing.T) {
	server, err := newServer()
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}

	// GET /users/999 → 404
	notFoundResp := serve(t, server, http.MethodGet, "/users/999", nil)
	if notFoundResp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /users/999 status = %d, want %d", notFoundResp.StatusCode, http.StatusNotFound)
	}

	// GET /users/abc → 400
	badIDResp := serve(t, server, http.MethodGet, "/users/abc", nil)
	if badIDResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("GET /users/abc status = %d, want %d", badIDResp.StatusCode, http.StatusBadRequest)
	}

	// PUT /users/999 → 404
	putNotFoundResp := serve(t, server, http.MethodPut, "/users/999", bytes.NewBufferString(`{"name":"Test","email":"test@example.com"}`))
	if putNotFoundResp.StatusCode != http.StatusNotFound {
		t.Fatalf("PUT /users/999 status = %d, want %d", putNotFoundResp.StatusCode, http.StatusNotFound)
	}

	// DELETE /users/999 → 404
	delNotFoundResp := serve(t, server, http.MethodDelete, "/users/999", nil)
	if delNotFoundResp.StatusCode != http.StatusNotFound {
		t.Fatalf("DELETE /users/999 status = %d, want %d", delNotFoundResp.StatusCode, http.StatusNotFound)
	}

	// POST with invalid JSON → 400
	badJSONResp := serve(t, server, http.MethodPost, "/users", bytes.NewBufferString(`{invalid}`))
	if badJSONResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST /users (invalid JSON) status = %d, want %d", badJSONResp.StatusCode, http.StatusBadRequest)
	}

	// POST with empty body → 400
	emptyBodyResp := serve(t, server, http.MethodPost, "/users", bytes.NewBufferString(``))
	if emptyBodyResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST /users (empty body) status = %d, want %d", emptyBodyResp.StatusCode, http.StatusBadRequest)
	}

	// POST with missing required field → 400
	missingNameResp := serve(t, server, http.MethodPost, "/users", bytes.NewBufferString(`{"email":"test@example.com"}`))
	if missingNameResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST /users (missing name) status = %d, want %d", missingNameResp.StatusCode, http.StatusBadRequest)
	}
}

func serve(t *testing.T, server interface {
	ServeHTTP(*http.Request) (*http.Response, error)
}, method, path string, body *bytes.Buffer,
) *http.Response {
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
