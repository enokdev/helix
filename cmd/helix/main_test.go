package main

import (
	"os"
	"path/filepath"
	"testing"
)

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
