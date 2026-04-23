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

func TestRunDBMigrateCreate(t *testing.T) {
	t.Parallel()

	dir := newCLIGenerateFixture(t)

	if err := run([]string{"db", "migrate", "create", "add_users", "--dir", dir}); err != nil {
		t.Fatalf("run(db migrate create) error = %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "db", "migrations", "*_add_users.go"))
	if err != nil {
		t.Fatalf("glob migration: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("migration matches = %v, want one add_users migration", matches)
	}
}

func TestRunDBMigrateUpDownStatus(t *testing.T) {
	dir := newCLIGenerateFixture(t)
	dbPath := filepath.Join(dir, "app.db")
	writeCLIFile(t, filepath.Join(dir, "config"), "application.yaml", "database:\n  url: sqlite://"+dbPath+"\n")
	writeCLIFile(t, filepath.Join(dir, "db", "migrations"), "20260422143000_create_users.go", `//go:build helixmigration

package main

import (
	"context"
	"database/sql"
)

func Up(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY)")
	return err
}

func Down(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, "DROP TABLE users")
	return err
}
`)

	if err := run([]string{"db", "migrate", "status", "--dir", dir}); err != nil {
		t.Fatalf("run(db migrate status) error = %v", err)
	}
	if err := run([]string{"db", "migrate", "up", "--dir", dir}); err != nil {
		t.Fatalf("run(db migrate up) error = %v", err)
	}
	if err := run([]string{"db", "migrate", "down", "--dir", dir}); err != nil {
		t.Fatalf("run(db migrate down) error = %v", err)
	}
}

func TestRunDBMigrateUsesDatabaseURLFlag(t *testing.T) {
	dir := newCLIGenerateFixture(t)
	dbPath := filepath.Join(dir, "flag.db")
	writeCLIFile(t, filepath.Join(dir, "db", "migrations"), "20260422143000_create_users.go", `//go:build helixmigration

package main

import (
	"context"
	"database/sql"
)

func Up(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY)")
	return err
}

func Down(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, "DROP TABLE users")
	return err
}
`)

	if err := run([]string{"db", "migrate", "up", "--dir", dir, "--database-url", "sqlite://" + dbPath}); err != nil {
		t.Fatalf("run(db migrate up --database-url) error = %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database stat error = %v", err)
	}
}

func TestRunDBMigrateErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing migrate", args: []string{"db"}, want: "expected subcommand migrate"},
		{name: "missing action", args: []string{"db", "migrate"}, want: "expected subcommand create, up, down, or status"},
		{name: "missing create name", args: []string{"db", "migrate", "create"}, want: "expected migration name"},
		{name: "unknown action", args: []string{"db", "migrate", "sideways"}, want: "expected subcommand create, up, down, or status"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := run(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("run(%v) error = %v, want %q", tt.args, err, tt.want)
			}
		})
	}
}

func TestRunRootCommandErrorsMentionRunAndBuild(t *testing.T) {
	t.Parallel()

	err := run(nil)
	if err == nil || !strings.Contains(err.Error(), "expected subcommand new, db, generate, run, or build") {
		t.Fatalf("run(nil) error = %v", err)
	}
}

func TestParseRunOptionsPassesAppArgs(t *testing.T) {
	t.Parallel()

	opts, err := parseRunOptions([]string{"--dir", "/tmp/service", "--", "--port=8080", "--env=dev"})
	if err != nil {
		t.Fatalf("parseRunOptions() error = %v", err)
	}
	if opts.Dir != "/tmp/service" {
		t.Fatalf("opts.Dir = %q", opts.Dir)
	}
	if got := strings.Join(opts.Args, " "); got != "--port=8080 --env=dev" {
		t.Fatalf("opts.Args = %q", got)
	}
}

func TestParseBuildOptionsRejectsUnexpectedArgs(t *testing.T) {
	t.Parallel()

	_, err := parseBuildOptions([]string{"--dir", "/tmp/service", "extra"})
	if err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("parseBuildOptions() error = %v, want unexpected argument", err)
	}
}

func TestRunBuildCreatesBinaryAndDockerfile(t *testing.T) {
	dir := newMinimalBuildFixture(t)

	if err := run([]string{"build", "--dir", dir, "--docker"}); err != nil {
		t.Fatalf("run(build --docker) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bin", "app")); err != nil {
		t.Fatalf("bin/app stat error = %v", err)
	}
	content := readCLIFile(t, filepath.Join(dir, "Dockerfile"))
	if !strings.Contains(content, "FROM scratch") {
		t.Fatalf("Dockerfile missing runtime stage:\n%s", content)
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

// newMinimalBuildFixture creates a minimal Go project with a cmd/app/main.go.
// Note: This fixture is suitable for `helix build` command testing only.
// It creates a compilable binary but does not include Helix framework code,
// so tests validate build tools exist, not actual Helix app correctness.
func newMinimalBuildFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	writeCLIFile(t, dir, "go.mod", "module example.test/clirun\n\ngo 1.21.0\n")
	writeCLIFile(t, filepath.Join(dir, "cmd", "app"), "main.go", `package main

import "fmt"

func main() {
	fmt.Println("hello")
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
