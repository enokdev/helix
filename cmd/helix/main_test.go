package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunNewApp(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	if err := run([]string{"new", "app", "my-service", "--dir", root}); err != nil {
		t.Fatalf("run(new app) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "my-service", "go.mod")); err != nil {
		t.Fatalf("generated go.mod stat error = %v", err)
	}
}

func TestRunGenerateModule(t *testing.T) {
	t.Parallel()

	dir := newCLIGenerateFixture(t)

	if err := run([]string{"generate", "module", "user", "--dir", dir}); err != nil {
		t.Fatalf("run(generate module) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "users", "service.go")); err != nil {
		t.Fatalf("users/service.go stat error = %v", err)
	}
}

func TestRunGenerateContext(t *testing.T) {
	t.Parallel()

	dir := newCLIGenerateFixture(t)

	if err := run([]string{"generate", "context", "accounts", "--dir", dir}); err != nil {
		t.Fatalf("run(generate context) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "accounts", "api.go")); err != nil {
		t.Fatalf("accounts/api.go stat error = %v", err)
	}
}

func TestRunGenerateContextThenWireCreatesWireFile(t *testing.T) {
	t.Parallel()

	dir := newCLIGenerateFixture(t)

	if err := run([]string{"generate", "context", "accounts", "--dir", dir}); err != nil {
		t.Fatalf("run(generate context) error = %v", err)
	}
	if err := run([]string{"generate", "wire", "--dir", dir}); err != nil {
		t.Fatalf("run(generate wire) error = %v", err)
	}
	wire := readCLIFile(t, filepath.Join(dir, "helix_wire_gen.go"))
	for _, want := range []string{"AccountRepository", "AccountService", "AccountController"} {
		if !strings.Contains(wire, want) {
			t.Fatalf("helix_wire_gen.go missing %q:\n%s", want, wire)
		}
	}
	if strings.Contains(wire, `"reflect"`) {
		t.Fatalf("helix_wire_gen.go unexpectedly imports reflect:\n%s", wire)
	}
}

func TestRunGenerateWireCreatesWireFile(t *testing.T) {
	t.Parallel()

	dir := newCLIGenerateFixture(t)

	if err := run([]string{"generate", "wire", "--dir", dir}); err != nil {
		t.Fatalf("run(generate wire) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "helix_wire_gen.go")); err != nil {
		t.Fatalf("helix_wire_gen.go stat error = %v", err)
	}
}

func TestRunGenerateWithoutWireKeepsExistingBehavior(t *testing.T) {
	t.Parallel()

	dir := newCLIGenerateFixture(t)

	if err := run([]string{"generate", "--dir", dir}); err != nil {
		t.Fatalf("run(generate) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "helix_wire_gen.go")); !os.IsNotExist(err) {
		t.Fatalf("helix_wire_gen.go stat error = %v, want not exist", err)
	}
}

func TestRunGenerateModuleRequiresName(t *testing.T) {
	t.Parallel()

	err := run([]string{"generate", "module"})
	if err == nil {
		t.Fatal("run(generate module) error = nil, want missing name")
	}
	if !strings.Contains(err.Error(), "expected module name") {
		t.Fatalf("run(generate module) error = %q, want expected module name", err)
	}
}

func TestRunGenerateContextRequiresName(t *testing.T) {
	t.Parallel()

	err := run([]string{"generate", "context"})
	if err == nil {
		t.Fatal("run(generate context) error = nil, want missing name")
	}
	if !strings.Contains(err.Error(), "expected context name") {
		t.Fatalf("run(generate context) error = %q, want expected context name", err)
	}
}

func newCLIGenerateFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	writeCLIFile(t, dir, "go.mod", "module example.test/cliwire\n\ngo 1.21.0\n")
	writeCLIFile(t, dir, "app.go", `package app

import "github.com/enokdev/helix"

type Repository struct {
	helix.Repository
}
`)
	return dir
}

func writeCLIFile(t *testing.T, dir, name, content string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func readCLIFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
