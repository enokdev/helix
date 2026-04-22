package migrate

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"go/format"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"
	"unicode"

	helixconfig "github.com/enokdev/helix/config"
	_ "github.com/mattn/go-sqlite3"
)

const (
	migrationDir  = "db/migrations"
	timestampForm = "20060102150405"
)

var (
	// ErrInvalidName reports an invalid migration name.
	ErrInvalidName = errors.New("invalid migration name")

	errInvalidRoot        = errors.New("invalid project root")
	errMissingDatabaseURL = errors.New("missing database url")
	errUnsupportedDB      = errors.New("unsupported database url")
	errDuplicateVersion   = errors.New("duplicate migration version")

	migrationFilePattern = regexp.MustCompile(`^([0-9]{14})_([a-z0-9_]+)\.go$`)
	sqlite3VersionRE     = regexp.MustCompile(`\bgithub\.com/mattn/go-sqlite3\s+(v\S+)`)
)

// Options configures migration status and execution commands.
type Options struct {
	RootDir     string
	DatabaseURL string
	Output      io.Writer
}

// CreateOptions configures migration file creation.
type CreateOptions struct {
	RootDir string
	Name    string
	Now     func() time.Time
}

type migration struct {
	Version string
	Name    string
	Path    string
}

type databaseTarget struct {
	Driver string
	DSN    string
}

type appConfig struct {
	Database struct {
		URL string `mapstructure:"url"`
	} `mapstructure:"database"`
}

type migrationTemplateData struct {
	Version string
	Name    string
}

// Create creates a timestamped Go migration file under db/migrations.
func Create(ctx context.Context, opts CreateOptions) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	root, err := projectRoot(opts.RootDir)
	if err != nil {
		return fmt.Errorf("migrate: create %s: %w", opts.Name, err)
	}
	name, err := normalizeName(opts.Name)
	if err != nil {
		return fmt.Errorf("migrate: create %s: %w", opts.Name, err)
	}
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	version := now().UTC().Format(timestampForm)
	content, err := renderMigrationTemplate(migrationTemplateData{Version: version, Name: name})
	if err != nil {
		return fmt.Errorf("migrate: create %s: %w", name, err)
	}
	path, err := safeJoin(root, filepath.Join(migrationDir, version+"_"+name+".go"))
	if err != nil {
		return fmt.Errorf("migrate: create %s: %w", name, err)
	}
	if err := writeNewFile(path, []byte(content)); err != nil {
		return fmt.Errorf("migrate: create %s: write %s: %w", name, filepath.ToSlash(filepath.Join(migrationDir, filepath.Base(path))), err)
	}
	return nil
}

// Up applies all pending migrations in chronological order.
func Up(ctx context.Context, opts Options) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	root, db, target, err := prepare(ctx, opts)
	if err != nil {
		return fmt.Errorf("migrate: up: %w", err)
	}

	migrations, err := discover(root)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("migrate: up: %w", err)
	}
	applied, err := appliedMigrations(ctx, db)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("migrate: up: %w", err)
	}
	_ = db.Close() // close journal before spawning subprocess

	pending := 0
	for _, m := range migrations {
		if _, ok := applied[m.Version]; ok {
			continue
		}
		pending++
		if err := runMigration(ctx, root, target, "up", m); err != nil {
			return fmt.Errorf("migrate: up %s: %w", m.Version, err)
		}
		fmt.Fprintf(output(opts.Output), "applied %s %s\n", m.Version, m.Name)
	}
	if pending == 0 {
		fmt.Fprintln(output(opts.Output), "no pending migrations")
	}
	return nil
}

// Down rolls back the latest applied migration.
func Down(ctx context.Context, opts Options) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	root, db, target, err := prepare(ctx, opts)
	if err != nil {
		return fmt.Errorf("migrate: down: %w", err)
	}

	latest, ok, err := latestApplied(ctx, db)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("migrate: down: %w", err)
	}
	if !ok {
		_ = db.Close()
		fmt.Fprintln(output(opts.Output), "no applied migrations")
		return nil
	}
	migrations, err := discover(root)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("migrate: down: %w", err)
	}
	found, ok := migrationByVersion(migrations, latest.Version)
	if !ok {
		_ = db.Close()
		return fmt.Errorf("migrate: down %s: migration file not found", latest.Version)
	}
	_ = db.Close() // close journal before spawning subprocess

	if err := runMigration(ctx, root, target, "down", found); err != nil {
		return fmt.Errorf("migrate: down %s: %w", found.Version, err)
	}
	fmt.Fprintf(output(opts.Output), "rolled back %s %s\n", found.Version, found.Name)
	return nil
}

// Status prints all discovered migrations and their applied/pending state.
// If no database is configured, all migrations are shown as pending.
func Status(ctx context.Context, opts Options) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	root, err := projectRoot(opts.RootDir)
	if err != nil {
		return fmt.Errorf("migrate: status: %w", err)
	}
	migrations, err := discover(root)
	if err != nil {
		return fmt.Errorf("migrate: status: %w", err)
	}
	applied, err := loadApplied(ctx, opts.DatabaseURL, root)
	if err != nil {
		return fmt.Errorf("migrate: status: %w", err)
	}
	out := output(opts.Output)
	fmt.Fprintln(out, "VERSION NAME STATUS")
	seen := make(map[string]struct{}, len(migrations))
	for _, m := range migrations {
		status := "pending"
		if _, ok := applied[m.Version]; ok {
			status = "applied"
		}
		seen[m.Version] = struct{}{}
		fmt.Fprintf(out, "%s %s %s\n", m.Version, m.Name, status)
	}
	var appliedOnly []migration
	for version, name := range applied {
		if _, ok := seen[version]; !ok {
			appliedOnly = append(appliedOnly, migration{Version: version, Name: name})
		}
	}
	sort.Slice(appliedOnly, func(i, j int) bool {
		return appliedOnly[i].Version < appliedOnly[j].Version
	})
	for _, m := range appliedOnly {
		fmt.Fprintf(out, "%s %s applied\n", m.Version, m.Name)
	}
	return nil
}

func prepare(ctx context.Context, opts Options) (string, *sql.DB, databaseTarget, error) {
	root, err := projectRoot(opts.RootDir)
	if err != nil {
		return "", nil, databaseTarget{}, err
	}
	target, err := resolveDatabaseTarget(opts.DatabaseURL, root)
	if err != nil {
		return "", nil, databaseTarget{}, err
	}
	db, err := sql.Open(target.Driver, target.DSN)
	if err != nil {
		return "", nil, databaseTarget{}, err
	}
	if err := ensureJournal(ctx, db); err != nil {
		_ = db.Close()
		return "", nil, databaseTarget{}, err
	}
	return root, db, target, nil
}

// loadApplied returns the set of applied migrations from the journal.
// If no database URL is configured, it returns an empty map (all migrations shown as pending).
func loadApplied(ctx context.Context, databaseURL, root string) (map[string]string, error) {
	target, err := resolveDatabaseTarget(databaseURL, root)
	if errors.Is(err, errMissingDatabaseURL) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(target.Driver, target.DSN)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := ensureJournal(ctx, db); err != nil {
		return nil, err
	}
	return appliedMigrations(ctx, db)
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("migrate: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func projectRoot(root string) (string, error) {
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	if _, err := os.Stat(filepath.Join(abs, "go.mod")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("go.mod not found: %w", errInvalidRoot)
		}
		return "", fmt.Errorf("stat go.mod: %w", err)
	}
	return abs, nil
}

func resolveDatabaseTarget(overrideURL, root string) (databaseTarget, error) {
	url := strings.TrimSpace(overrideURL)
	if url == "" {
		var cfg appConfig
		loader := helixconfig.NewLoader(
			helixconfig.WithConfigPaths(filepath.Join(root, "config")),
			helixconfig.WithAllowMissingConfig(),
		)
		if err := loader.Load(&cfg); err != nil {
			return databaseTarget{}, err
		}
		url = strings.TrimSpace(cfg.Database.URL)
	}
	if url == "" {
		return databaseTarget{}, errMissingDatabaseURL
	}
	switch {
	case strings.HasPrefix(url, "sqlite://"):
		dsn := strings.TrimPrefix(url, "sqlite://")
		if dsn == "" {
			return databaseTarget{}, errUnsupportedDB
		}
		return databaseTarget{Driver: "sqlite3", DSN: dsn}, nil
	case strings.HasPrefix(url, "sqlite3://"):
		dsn := strings.TrimPrefix(url, "sqlite3://")
		if dsn == "" {
			return databaseTarget{}, errUnsupportedDB
		}
		return databaseTarget{Driver: "sqlite3", DSN: dsn}, nil
	case strings.HasPrefix(url, "file:"), url == ":memory:", strings.HasSuffix(url, ".db"), strings.HasSuffix(url, ".sqlite"), strings.HasSuffix(url, ".sqlite3"):
		return databaseTarget{Driver: "sqlite3", DSN: url}, nil
	default:
		return databaseTarget{}, fmt.Errorf("%w: only sqlite URLs are supported", errUnsupportedDB)
	}
}

func ensureJournal(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS helix_migrations (
	version TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at TIMESTAMP NOT NULL
)`)
	if err != nil {
		return fmt.Errorf("create helix_migrations: %w", err)
	}
	return nil
}

func appliedMigrations(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT version, name FROM helix_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()
	applied := make(map[string]string)
	for rows.Next() {
		var version, name string
		if err := rows.Scan(&version, &name); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = name
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return applied, nil
}

func latestApplied(ctx context.Context, db *sql.DB) (migration, bool, error) {
	var m migration
	err := db.QueryRowContext(ctx, "SELECT version, name FROM helix_migrations ORDER BY version DESC LIMIT 1").Scan(&m.Version, &m.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return migration{}, false, nil
	}
	if err != nil {
		return migration{}, false, fmt.Errorf("latest applied migration: %w", err)
	}
	return m, true, nil
}

func discover(root string) ([]migration, error) {
	dir, err := safeJoin(root, migrationDir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var migrations []migration
	seen := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationFilePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		version, name := matches[1], matches[2]
		if previous, ok := seen[version]; ok {
			return nil, fmt.Errorf("%w %s (%s, %s)", errDuplicateVersion, version, previous, entry.Name())
		}
		seen[version] = entry.Name()
		migrations = append(migrations, migration{
			Version: version,
			Name:    name,
			Path:    filepath.Join(dir, entry.Name()),
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

func migrationByVersion(migrations []migration, version string) (migration, bool) {
	for _, m := range migrations {
		if m.Version == version {
			return m, true
		}
	}
	return migration{}, false
}

func runMigration(ctx context.Context, root string, target databaseTarget, action string, m migration) error {
	tempDir, err := os.MkdirTemp("", "helix-migration-*")
	if err != nil {
		return fmt.Errorf("create temp runner: %w", err)
	}
	defer os.RemoveAll(tempDir)

	source, err := os.ReadFile(m.Path)
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}
	files := map[string]string{
		"go.mod":       runnerGoMod(sqlite3VersionFromGoMod(root)),
		"migration.go": string(source),
		"runner.go":    runnerSource(),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tempDir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write temp %s: %w", name, err)
		}
	}

	cmd := exec.CommandContext(ctx, "go", "run", "-tags", "helixmigration", ".", action, target.Driver, target.DSN, m.Version, m.Name)
	cmd.Dir = tempDir
	cmd.Env = append(filterEnv("GOFLAGS", "GOWORK"), "GOWORK=off", "GOFLAGS=-mod=mod")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run migration %s from %s: %w\n%s", action, filepath.ToSlash(m.Path), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func runnerGoMod(sqlite3Version string) string {
	return "module helix-migration-runner\n\ngo 1.21.0\n\nrequire github.com/mattn/go-sqlite3 " + sqlite3Version + "\n"
}

// sqlite3VersionFromGoMod reads the go-sqlite3 version from the project go.mod.
// Falls back to v1.14.22 if not found.
func sqlite3VersionFromGoMod(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err == nil {
		if m := sqlite3VersionRE.FindSubmatch(data); m != nil {
			return string(m[1])
		}
	}
	return "v1.14.22"
}

// filterEnv returns os.Environ() with all entries whose key matches any of the provided keys removed.
func filterEnv(keys ...string) []string {
	remove := make(map[string]bool, len(keys))
	for _, k := range keys {
		remove[strings.ToUpper(k)] = true
	}
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		key := strings.ToUpper(strings.SplitN(e, "=", 2)[0])
		if !remove[key] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func runnerSource() string {
	return `package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	if len(os.Args) != 6 {
		fail(fmt.Errorf("usage: runner <up|down> <driver> <dsn> <version> <name>"))
	}
	action, driver, dsn, version, name := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5]
	ctx := context.Background()
	db, err := sql.Open(driver, dsn)
	if err != nil {
		fail(err)
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		fail(err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	switch action {
	case "up":
		if err := Up(ctx, tx); err != nil {
			fail(err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO helix_migrations (version, name, applied_at) VALUES (?, ?, CURRENT_TIMESTAMP)", version, name); err != nil {
			fail(err)
		}
	case "down":
		if err := Down(ctx, tx); err != nil {
			fail(err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM helix_migrations WHERE version = ?", version); err != nil {
			fail(err)
		}
	default:
		fail(fmt.Errorf("unknown action %q", action))
	}
	if err := tx.Commit(); err != nil {
		fail(err)
	}
	committed = true
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
`
}

func renderMigrationTemplate(data migrationTemplateData) (string, error) {
	var buf bytes.Buffer
	if err := template.Must(template.New("migration").Parse(migrationTemplate)).Execute(&buf, data); err != nil {
		return "", err
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("format migration template: %w", err)
	}
	return string(formatted), nil
}

const migrationTemplate = `//go:build helixmigration
// +build helixmigration

package main

import (
	"context"
	"database/sql"
)

const migrationVersion = "{{ .Version }}"
const migrationName = "{{ .Name }}"

func Up(ctx context.Context, tx *sql.Tx) error {
	return nil
}

func Down(ctx context.Context, tx *sql.Tx) error {
	return nil
}
`

func normalizeName(name string) (string, error) {
	name = strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		return "", ErrInvalidName
	}
	if strings.HasPrefix(name, "_") || strings.HasSuffix(name, "_") || strings.Contains(name, "__") {
		return "", ErrInvalidName
	}
	for _, r := range name {
		if r == '_' || unicode.IsDigit(r) || ('a' <= r && r <= 'z') {
			continue
		}
		return "", ErrInvalidName
	}
	if unicode.IsDigit([]rune(name)[0]) {
		return "", ErrInvalidName
	}
	return name, nil
}

func safeJoin(root, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", fmt.Errorf("invalid path %q", name)
	}
	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	joined := filepath.Join(cleanRoot, filepath.Clean(name))
	rel, err := filepath.Rel(cleanRoot, joined)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root %q", name, root)
	}
	return joined, nil
}

func writeNewFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("refusing to overwrite existing file %s", filepath.Base(path))
		}
		return err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func output(out io.Writer) io.Writer {
	if out == nil {
		return io.Discard
	}
	return out
}
