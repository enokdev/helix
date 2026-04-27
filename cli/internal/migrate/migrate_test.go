package migrate

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestCreateMigrationWritesTaggedGoFile(t *testing.T) {
	t.Parallel()

	root := newProjectFixture(t)
	createdAt := time.Date(2026, 4, 22, 15, 30, 0, 0, time.FixedZone("test", 3600))

	if err := Create(context.Background(), CreateOptions{
		RootDir: root,
		Name:    "add_users_table",
		Now:     func() time.Time { return createdAt },
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	path := filepath.Join(root, "db", "migrations", "20260422143000_add_users_table.go")
	content := readFile(t, path)
	for _, want := range []string{
		"//go:build helixmigration",
		"package main",
		"func Up(ctx context.Context, tx *sql.Tx) error",
		"func Down(ctx context.Context, tx *sql.Tx) error",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("migration file missing %q:\n%s", want, content)
		}
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated project go test error = %v\n%s", err, output)
	}
}

func TestCreateMigrationRejectsInvalidNames(t *testing.T) {
	t.Parallel()

	root := newProjectFixture(t)
	for _, name := range []string{"", ".", "..", "add/users", "add users", "add__users", "_add_users", "add_users_"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := Create(context.Background(), CreateOptions{RootDir: root, Name: name})
			if !errors.Is(err, ErrInvalidName) {
				t.Fatalf("Create(%q) error = %v, want ErrInvalidName", name, err)
			}
		})
	}
}

func TestCreateMigrationNormalizesUppercaseAndDashes(t *testing.T) {
	t.Parallel()

	root := newProjectFixture(t)
	now := func() time.Time { return time.Date(2026, 4, 22, 15, 30, 0, 0, time.UTC) }

	if err := Create(context.Background(), CreateOptions{RootDir: root, Name: "AddUsers", Now: now}); err != nil {
		t.Fatalf("Create(AddUsers) error = %v", err)
	}
	path := filepath.Join(root, "db", "migrations", "20260422153000_addusers.go")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
}

func TestCreateMigrationRefusesOverwrite(t *testing.T) {
	t.Parallel()

	root := newProjectFixture(t)
	now := func() time.Time { return time.Date(2026, 4, 22, 15, 30, 0, 0, time.UTC) }
	opts := CreateOptions{RootDir: root, Name: "add_users", Now: now}

	if err := Create(context.Background(), opts); err != nil {
		t.Fatalf("Create() first error = %v", err)
	}
	if err := Create(context.Background(), opts); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("Create() second error = %v, want overwrite refusal", err)
	}
}

func TestStatusWithoutDatabase(t *testing.T) {
	t.Parallel()

	root := newProjectFixture(t)
	writeMigration(t, root, "20260422143000_create_users.go", "users")
	writeMigration(t, root, "20260422143100_create_posts.go", "posts")

	var buf bytes.Buffer
	if err := Status(context.Background(), Options{RootDir: root, Output: &buf}); err != nil {
		t.Fatalf("Status() without DB error = %v", err)
	}
	assertLines(t, buf.String(), []string{
		"VERSION NAME STATUS",
		"20260422143000 create_users pending",
		"20260422143100 create_posts pending",
	})
}

func TestUpDownAndStatusSQLite(t *testing.T) {
	root := newProjectFixture(t)
	dbPath := filepath.Join(root, "app.db")
	databaseURL := "sqlite://" + dbPath
	writeConfig(t, root, databaseURL)
	writeMigration(t, root, "20260422143000_create_users.go", "users")
	writeMigration(t, root, "20260422143100_create_posts.go", "posts")

	var status bytes.Buffer
	if err := Status(context.Background(), Options{RootDir: root, Output: &status}); err != nil {
		t.Fatalf("Status() before up error = %v", err)
	}
	assertLines(t, status.String(), []string{
		"VERSION NAME STATUS",
		"20260422143000 create_users pending",
		"20260422143100 create_posts pending",
	})

	var upOut bytes.Buffer
	if err := Up(context.Background(), Options{RootDir: root, Output: &upOut}); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	assertTableExists(t, dbPath, "users", true)
	assertTableExists(t, dbPath, "posts", true)
	assertMigrationCount(t, dbPath, 2)

	status.Reset()
	if err := Status(context.Background(), Options{RootDir: root, Output: &status}); err != nil {
		t.Fatalf("Status() after up error = %v", err)
	}
	assertLines(t, status.String(), []string{
		"VERSION NAME STATUS",
		"20260422143000 create_users applied",
		"20260422143100 create_posts applied",
	})

	var downOut bytes.Buffer
	if err := Down(context.Background(), Options{RootDir: root, Output: &downOut}); err != nil {
		t.Fatalf("Down() error = %v", err)
	}
	assertTableExists(t, dbPath, "users", true)
	assertTableExists(t, dbPath, "posts", false)
	assertMigrationCount(t, dbPath, 1)
}

func TestUpUsesDatabaseURLOverride(t *testing.T) {
	root := newProjectFixture(t)
	dbPath := filepath.Join(root, "override.db")
	writeMigration(t, root, "20260422143000_create_users.go", "users")

	if err := Up(context.Background(), Options{RootDir: root, DatabaseURL: "sqlite://" + dbPath}); err != nil {
		t.Fatalf("Up() with DatabaseURL override error = %v", err)
	}
	assertTableExists(t, dbPath, "users", true)
	assertMigrationCount(t, dbPath, 1)
}

func TestUpStopsOnFailedMigration(t *testing.T) {
	root := newProjectFixture(t)
	dbPath := filepath.Join(root, "app.db")
	databaseURL := "sqlite://" + dbPath
	writeConfig(t, root, databaseURL)
	writeMigration(t, root, "20260422143000_create_users.go", "users")
	writeFailingMigration(t, root, "20260422143100_create_posts.go")
	writeMigration(t, root, "20260422143200_create_comments.go", "comments")

	err := Up(context.Background(), Options{RootDir: root})
	if err == nil || !strings.Contains(err.Error(), "20260422143100") {
		t.Fatalf("Up() error = %v, want failing migration version", err)
	}
	assertTableExists(t, dbPath, "users", true)
	assertTableExists(t, dbPath, "comments", false)
	assertMigrationCount(t, dbPath, 1)
}

func TestStatusRejectsDuplicateMigrationVersions(t *testing.T) {
	root := newProjectFixture(t)
	dbPath := filepath.Join(root, "app.db")
	writeConfig(t, root, "sqlite://"+dbPath)
	writeMigration(t, root, "20260422143000_create_users.go", "users")
	writeMigration(t, root, "20260422143000_create_posts.go", "posts")

	err := Status(context.Background(), Options{RootDir: root})
	if !errors.Is(err, errDuplicateVersion) {
		t.Fatalf("Status() error = %v, want errDuplicateVersion", err)
	}
}

func TestRunnerSourceReadsDSNFromEnvironment(t *testing.T) {
	source := runnerSource()

	for _, forbidden := range []string{
		"<dsn>",
		"dsn := os.Args",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("runnerSource() must not read DSN from argv; found %q in:\n%s", forbidden, source)
		}
	}
	for _, want := range []string{
		`os.Getenv("` + migrationDSNEnv + `")`,
		"missing migration DSN",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("runnerSource() missing %q in:\n%s", want, source)
		}
	}
}

func newProjectFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.test/app\n\ngo 1.21.0\n")
	writeFile(t, root, "app.go", "package app\n")
	return root
}

func writeConfig(t *testing.T, root, databaseURL string) {
	t.Helper()
	writeFile(t, root, filepath.Join("config", "application.yaml"), "database:\n  url: "+databaseURL+"\n")
}

func writeMigration(t *testing.T, root, name, table string) {
	t.Helper()
	writeFile(t, root, filepath.Join("db", "migrations", name), `//go:build helixmigration

package main

import (
	"context"
	"database/sql"
)

func Up(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, "CREATE TABLE `+table+` (id INTEGER PRIMARY KEY)")
	return err
}

func Down(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, "DROP TABLE `+table+`")
	return err
}
`)
}

func writeFailingMigration(t *testing.T, root, name string) {
	t.Helper()
	writeFile(t, root, filepath.Join("db", "migrations", name), `//go:build helixmigration

package main

import (
	"context"
	"database/sql"
	"fmt"
)

func Up(ctx context.Context, tx *sql.Tx) error {
	return fmt.Errorf("boom")
}

func Down(ctx context.Context, tx *sql.Tx) error {
	return nil
}
`)
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertLines(t *testing.T, got string, want []string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != len(want) {
		t.Fatalf("lines = %#v, want %#v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("line %d = %q, want %q\nall:\n%s", i, lines[i], want[i], got)
		}
	}
}

func assertTableExists(t *testing.T, dbPath, table string, want bool) {
	t.Helper()
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	var name string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
	got := err == nil
	if got != want {
		t.Fatalf("table %s exists = %v, want %v, err=%v", table, got, want, err)
	}
}

func assertMigrationCount(t *testing.T, dbPath string, want int) {
	t.Helper()
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	var got int
	if err := db.QueryRow("SELECT COUNT(*) FROM helix_migrations").Scan(&got); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if got != want {
		t.Fatalf("helix_migrations count = %d, want %d", got, want)
	}
}
