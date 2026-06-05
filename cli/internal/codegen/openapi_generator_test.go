package codegen

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAPIGeneratorGeneratesConventionalAndDirectiveRoutes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/app\n\ngo 1.21\n")
	writeFile(t, dir, "users.go", `package main

import (
	"github.com/enokdev/helix"
	"github.com/enokdev/helix/web"
)

type UserController struct {
	helix.Controller `+"`helix:\"route:/api/v1/users\"`"+`
}

func (c *UserController) Index() []string { return nil }
func (c *UserController) Show(ctx web.Context) (string, error) { return "", nil }

type AuthController struct {
	helix.Controller
}

//helix:route POST /auth/login
func (c *AuthController) Login(ctx web.Context) error { return nil }
`)

	output := filepath.Join(dir, "openapi.json")
	result, err := NewOpenAPIGenerator(dir, output).Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(result.Files) != 1 || result.Files[0] != output {
		t.Fatalf("generated files = %v, want [%s]", result.Files, output)
	}

	var doc map[string]any
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("openapi json invalid: %v\n%s", err, data)
	}
	paths := doc["paths"].(map[string]any)
	assertPathMethod(t, paths, "/api/v1/users", "get")
	assertPathMethod(t, paths, "/api/v1/users/{id}", "get")
	assertPathMethod(t, paths, "/auth/login", "post")
}

func assertPathMethod(t *testing.T, paths map[string]any, routePath, method string) {
	t.Helper()

	item, ok := paths[routePath].(map[string]any)
	if !ok {
		t.Fatalf("path %s missing from %#v", routePath, paths)
	}
	if _, ok := item[method].(map[string]any); !ok {
		t.Fatalf("method %s missing for path %s: %#v", method, routePath, item)
	}
}

func writeFile(t *testing.T, root, name, contents string) {
	t.Helper()

	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
