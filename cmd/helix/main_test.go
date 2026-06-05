package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunNewApp(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	if err := run(context.Background(), []string{"new", "app", "my-service", "--dir", root}); err != nil {
		t.Fatalf("run(new app) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "my-service", "go.mod")); err != nil {
		t.Fatalf("generated go.mod stat error = %v", err)
	}
}

func TestRunGenerateModule(t *testing.T) {
	t.Parallel()

	dir := newCLIGenerateFixture(t)

	if err := run(context.Background(), []string{"generate", "module", "user", "--dir", dir}); err != nil {
		t.Fatalf("run(generate module) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "users", "service.go")); err != nil {
		t.Fatalf("users/service.go stat error = %v", err)
	}
}

func TestRunGenerateContext(t *testing.T) {
	t.Parallel()

	dir := newCLIGenerateFixture(t)

	if err := run(context.Background(), []string{"generate", "context", "accounts", "--dir", dir}); err != nil {
		t.Fatalf("run(generate context) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "accounts", "api.go")); err != nil {
		t.Fatalf("accounts/api.go stat error = %v", err)
	}
}

func TestRunGenerateOpenAPI(t *testing.T) {
	t.Parallel()

	dir := newCLIGenerateFixture(t)
	writeCLIFile(t, dir, "controller.go", `package main

import (
	"github.com/enokdev/helix"
	"github.com/enokdev/helix/web"
)

type UserController struct {
	helix.Controller `+"`helix:\"route:/api/users\"`"+`
}

func (c *UserController) Index() []string { return nil }
func (c *UserController) Show(ctx web.Context) (string, error) { return "", nil }
`)
	output := filepath.Join(dir, "public-openapi.json")

	if err := run(context.Background(), []string{"generate", "openapi", "--dir", dir, "--output", output}); err != nil {
		t.Fatalf("run(generate openapi) error = %v", err)
	}

	data := readCLIFile(t, output)
	var doc map[string]any
	if err := json.Unmarshal([]byte(data), &doc); err != nil {
		t.Fatalf("generated OpenAPI is invalid JSON: %v\n%s", err, data)
	}
	paths := doc["paths"].(map[string]any)
	if _, ok := paths["/api/users"].(map[string]any)["get"]; !ok {
		t.Fatalf("OpenAPI missing GET /api/users: %s", data)
	}
	if _, ok := paths["/api/users/{id}"].(map[string]any)["get"]; !ok {
		t.Fatalf("OpenAPI missing GET /api/users/{id}: %s", data)
	}
}

func TestRunGenerateContextThenWireCreatesWireFile(t *testing.T) {
	t.Parallel()

	dir := newCLIGenerateFixture(t)

	if err := run(context.Background(), []string{"generate", "context", "accounts", "--dir", dir}); err != nil {
		t.Fatalf("run(generate context) error = %v", err)
	}
	if err := run(context.Background(), []string{"generate", "wire", "--dir", dir}); err != nil {
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

	if err := run(context.Background(), []string{"db", "migrate", "create", "add_users", "--dir", dir}); err != nil {
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

	if err := run(context.Background(), []string{"db", "migrate", "status", "--dir", dir}); err != nil {
		t.Fatalf("run(db migrate status) error = %v", err)
	}
	if err := run(context.Background(), []string{"db", "migrate", "up", "--dir", dir}); err != nil {
		t.Fatalf("run(db migrate up) error = %v", err)
	}
	if err := run(context.Background(), []string{"db", "migrate", "down", "--dir", dir}); err != nil {
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

	if err := run(context.Background(), []string{"db", "migrate", "up", "--dir", dir, "--database-url", "sqlite://" + dbPath}); err != nil {
		t.Fatalf("run(db migrate up --database-url) error = %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database stat error = %v", err)
	}
}

func TestRunDBMigrateUpCGODisabledExplainsSQLiteRequirement(t *testing.T) {
	dir := newCLIGenerateFixture(t)
	dbPath := filepath.Join(dir, "app.db")
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
	t.Setenv("CGO_ENABLED", "0")

	err := run(context.Background(), []string{"db", "migrate", "up", "--dir", dir, "--database-url", "sqlite://" + dbPath})
	if err == nil {
		t.Fatal("run(db migrate up) error = nil, want CGo diagnostic")
	}
	for _, want := range []string{"cli: db migrate up", "go-sqlite3 requires CGo", "CGO_ENABLED=1"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("run(db migrate up) error = %q, want %q", err, want)
		}
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
			err := run(context.Background(), tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("run(%v) error = %v, want %q", tt.args, err, tt.want)
			}
		})
	}
}

func TestRunCommandsWithPositionalsAcceptFlagsBeforeAndAfterName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       func(root string) []string
		assertPath func(root string) string
	}{
		{
			name:       "new app flags before name",
			args:       func(root string) []string { return []string{"new", "app", "--dir", root, "my-service"} },
			assertPath: func(root string) string { return filepath.Join(root, "my-service", "go.mod") },
		},
		{
			name:       "new app flags after name",
			args:       func(root string) []string { return []string{"new", "app", "my-service", "--dir", root} },
			assertPath: func(root string) string { return filepath.Join(root, "my-service", "go.mod") },
		},
		{
			name:       "generate module flags before name",
			args:       func(root string) []string { return []string{"generate", "module", "--dir", root, "user"} },
			assertPath: func(root string) string { return filepath.Join(root, "users", "service.go") },
		},
		{
			name:       "generate module flags after name",
			args:       func(root string) []string { return []string{"generate", "module", "user", "--dir", root} },
			assertPath: func(root string) string { return filepath.Join(root, "users", "service.go") },
		},
		{
			name:       "generate context flags before name",
			args:       func(root string) []string { return []string{"generate", "context", "--dir", root, "accounts"} },
			assertPath: func(root string) string { return filepath.Join(root, "accounts", "api.go") },
		},
		{
			name:       "generate context flags after name",
			args:       func(root string) []string { return []string{"generate", "context", "accounts", "--dir", root} },
			assertPath: func(root string) string { return filepath.Join(root, "accounts", "api.go") },
		},
		{
			name:       "db migrate create flags before name",
			args:       func(root string) []string { return []string{"db", "migrate", "create", "--dir", root, "add_users"} },
			assertPath: func(root string) string { return filepath.Join(root, "db", "migrations") },
		},
		{
			name:       "db migrate create flags after name",
			args:       func(root string) []string { return []string{"db", "migrate", "create", "add_users", "--dir", root} },
			assertPath: func(root string) string { return filepath.Join(root, "db", "migrations") },
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := newCLIGenerateFixture(t)
			if strings.HasPrefix(tt.name, "new app") {
				root = t.TempDir()
			}

			if err := run(context.Background(), tt.args(root)); err != nil {
				t.Fatalf("run(%v) error = %v", tt.args(root), err)
			}
			if _, err := os.Stat(tt.assertPath(root)); err != nil {
				t.Fatalf("expected generated path stat error = %v", err)
			}
		})
	}
}

func TestRunCommandFlagErrorsIncludeCommandContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "new app unknown flag", args: []string{"new", "app", "my-service", "--unknown"}, want: "helix new app: flag provided but not defined: -unknown"},
		{name: "generate module unknown flag", args: []string{"generate", "module", "user", "--unknown"}, want: "helix generate module: flag provided but not defined: -unknown"},
		{name: "db migrate create unknown flag", args: []string{"db", "migrate", "create", "add_users", "--unknown"}, want: "helix db migrate create: flag provided but not defined: -unknown"},
		{name: "build unknown flag", args: []string{"build", "--unknown"}, want: "helix build: flag provided but not defined: -unknown"},
		{name: "generate unknown flag", args: []string{"generate", "--unknown"}, want: "helix generate: flag provided but not defined: -unknown"},
		{name: "db migrate up unknown flag", args: []string{"db", "migrate", "up", "--unknown"}, want: "helix db migrate up: flag provided but not defined: -unknown"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := run(context.Background(), tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("run(%v) error = %v, want %q", tt.args, err, tt.want)
			}
		})
	}
}

func TestRunGenerateReportsChangedFiles(t *testing.T) {
	dir := newCLIRepositoryGenerateFixture(t)
	var out bytes.Buffer
	oldStdout := cliStdout
	cliStdout = &out
	t.Cleanup(func() { cliStdout = oldStdout })

	if err := run(context.Background(), []string{"generate", "--dir", dir}); err != nil {
		t.Fatalf("run(generate) error = %v", err)
	}
	if got, want := strings.TrimSpace(out.String()), "generated user_repository_query_gen.go"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}

	out.Reset()
	if err := run(context.Background(), []string{"generate", "--dir", dir}); err != nil {
		t.Fatalf("second run(generate) error = %v", err)
	}
	if got, want := strings.TrimSpace(out.String()), "no generated file changes"; got != want {
		t.Fatalf("second stdout = %q, want %q", got, want)
	}
}

func TestRunGenerateUsesProvidedContext(t *testing.T) {
	t.Parallel()

	dir := newCLIRepositoryGenerateFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := run(ctx, []string{"generate", "--dir", dir})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run(generate) error = %v, want context.Canceled", err)
	}
}

func TestRunRootCommandErrorsMentionRunAndBuild(t *testing.T) {
	t.Parallel()

	err := run(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "expected subcommand new, db, generate, run, build, or version") {
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

	if err := run(context.Background(), []string{"build", "--dir", dir, "--docker"}); err != nil {
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

	if err := run(context.Background(), []string{"generate", "wire", "--dir", dir}); err != nil {
		t.Fatalf("run(generate wire) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "helix_wire_gen.go")); err != nil {
		t.Fatalf("helix_wire_gen.go stat error = %v", err)
	}
}

func TestRunGenerateWithoutWireKeepsExistingBehavior(t *testing.T) {
	t.Parallel()

	dir := newCLIGenerateFixture(t)

	if err := run(context.Background(), []string{"generate", "--dir", dir}); err != nil {
		t.Fatalf("run(generate) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "helix_wire_gen.go")); !os.IsNotExist(err) {
		t.Fatalf("helix_wire_gen.go stat error = %v, want not exist", err)
	}
}

func TestRunGenerateModuleRequiresName(t *testing.T) {
	t.Parallel()

	err := run(context.Background(), []string{"generate", "module"})
	if err == nil {
		t.Fatal("run(generate module) error = nil, want missing name")
	}
	if !strings.Contains(err.Error(), "expected module name") {
		t.Fatalf("run(generate module) error = %q, want expected module name", err)
	}
}

func TestRunGenerateContextRequiresName(t *testing.T) {
	t.Parallel()

	err := run(context.Background(), []string{"generate", "context"})
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

func newCLIRepositoryGenerateFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	writeCLIFile(t, dir, "go.mod", "module example.test/clirepository\n\ngo 1.21.0\n")
	writeCLIFile(t, dir, "repository.go", `package app

import (
	"context"

	"github.com/enokdev/helix/data"
)

type User struct {
	ID    int
	Email string
}

type UserRepository interface {
	data.Repository[User, int, int]

	//helix:query auto
	FindByEmail(ctx context.Context, email string) (*User, error)
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

func TestRunNewAPI(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	if err := run(context.Background(), []string{"new", "api", "my-api", "--dir", root}); err != nil {
		t.Fatalf("run(new api) error = %v", err)
	}
	appDir := filepath.Join(root, "my-api")
	for _, name := range []string{"go.mod", "main.go", "main_test.go", filepath.Join("config", "application.yaml")} {
		if _, err := os.Stat(filepath.Join(appDir, name)); err != nil {
			t.Fatalf("generated file %s stat error = %v", name, err)
		}
	}

	main := readCLIFile(t, filepath.Join(appDir, "main.go"))
	if !strings.Contains(main, "UserController") {
		t.Fatal("main.go missing UserController")
	}
}

func TestRunNewSecuredAPI(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	if err := run(context.Background(), []string{"new", "secured-api", "my-secured", "--dir", root}); err != nil {
		t.Fatalf("run(new secured-api) error = %v", err)
	}
	appDir := filepath.Join(root, "my-secured")
	for _, name := range []string{"go.mod", "main.go", "main_test.go", filepath.Join("config", "application.yaml")} {
		if _, err := os.Stat(filepath.Join(appDir, name)); err != nil {
			t.Fatalf("generated file %s stat error = %v", name, err)
		}
	}

	main := readCLIFile(t, filepath.Join(appDir, "main.go"))
	if !strings.Contains(main, "AuthController") {
		t.Fatal("main.go missing AuthController")
	}

	cfg := readCLIFile(t, filepath.Join(appDir, "config", "application.yaml"))
	if !strings.Contains(cfg, "jwt:") {
		t.Fatal("application.yaml missing jwt section")
	}
}

func TestRunNewGORMAPI(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	if err := run(context.Background(), []string{"new", "gorm-api", "my-gorm", "--dir", root}); err != nil {
		t.Fatalf("run(new gorm-api) error = %v", err)
	}
	appDir := filepath.Join(root, "my-gorm")
	for _, name := range []string{"go.mod", "main.go", "main_test.go", filepath.Join("config", "application.yaml")} {
		if _, err := os.Stat(filepath.Join(appDir, name)); err != nil {
			t.Fatalf("generated file %s stat error = %v", name, err)
		}
	}

	goMod := readCLIFile(t, filepath.Join(appDir, "go.mod"))
	if !strings.Contains(goMod, "gorm.io/driver/sqlite") {
		t.Fatal("go.mod missing gorm.io/driver/sqlite")
	}

	main := readCLIFile(t, filepath.Join(appDir, "main.go"))
	if !strings.Contains(main, "openDB") {
		t.Fatal("main.go missing openDB function")
	}
}

func TestRunNewUnknownSubcommand(t *testing.T) {
	t.Parallel()

	err := run(context.Background(), []string{"new", "unknown-template", "my-app", "--dir", t.TempDir()})
	if err == nil {
		t.Fatal("expected error for unknown template, got nil")
	}
	if !strings.Contains(err.Error(), "expected subcommand app, api, secured-api, or gorm-api") {
		t.Fatalf("error = %q, want subcommand list", err)
	}
}
