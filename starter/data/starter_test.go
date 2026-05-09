package data

import (
	"context"
	"errors"
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/enokdev/helix/core"
	datagorm "github.com/enokdev/helix/data/gorm"
)

// fakeConfig implements helixconfig.Loader for tests.
type fakeConfig struct {
	values map[string]any
}

func (f fakeConfig) Load(any) error              { return nil }
func (f fakeConfig) ConfigFileUsed() string      { return "" }
func (f fakeConfig) AllSettings() map[string]any { return f.values }
func (f fakeConfig) ActiveProfiles() []string    { return nil }

func (f fakeConfig) Lookup(key string) (any, bool) {
	v, ok := f.values[key]
	return v, ok
}

func newTestContainer() *core.Container {
	return core.NewContainer(core.WithResolver(core.NewReflectResolver()))
}

var cwdMu sync.Mutex

func chdirForTest(t *testing.T, dir string) {
	t.Helper()

	cwdMu.Lock()
	oldDir, err := os.Getwd()
	if err != nil {
		cwdMu.Unlock()
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		cwdMu.Unlock()
		t.Fatalf("chdir %q: %v", dir, err)
	}
	t.Cleanup(func() {
		defer cwdMu.Unlock()
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}

// chdirWithGoMod creates a temp dir with the given go.mod content and changes into it.
func chdirWithGoMod(t *testing.T, contents string) {
	t.Helper()

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(contents), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	chdirForTest(t, tmpDir)
}

func goModWithSQLite() string {
	return "module example.com/app\n\nrequire gorm.io/driver/sqlite v1.6.0\n"
}

func TestCondition(t *testing.T) {
	tests := []struct {
		name   string
		goMod  string
		cfg    fakeConfig
		useCfg bool
		want   bool
	}{
		{
			name:   "sqlite + database.url → active",
			goMod:  goModWithSQLite(),
			cfg:    fakeConfig{values: map[string]any{"database.url": ":memory:"}},
			useCfg: true,
			want:   true,
		},
		{
			name:  "override false → inactive",
			goMod: goModWithSQLite(),
			cfg: fakeConfig{values: map[string]any{
				"database.url":                ":memory:",
				"helix.starters.data.enabled": false,
			}},
			useCfg: true,
			want:   false,
		},
		{
			name:   "cfg nil → inactive",
			goMod:  goModWithSQLite(),
			useCfg: false,
			want:   false,
		},
		{
			name:   "database.url absent → inactive",
			goMod:  goModWithSQLite(),
			cfg:    fakeConfig{values: map[string]any{}},
			useCfg: true,
			want:   false,
		},
		{
			name:   "database.url empty → inactive",
			goMod:  goModWithSQLite(),
			cfg:    fakeConfig{values: map[string]any{"database.url": ""}},
			useCfg: true,
			want:   false,
		},
		{
			name:   "driver absent → inactive",
			goMod:  "module example.com/app\n\nrequire github.com/some/lib v1.0.0\n",
			cfg:    fakeConfig{values: map[string]any{"database.url": ":memory:"}},
			useCfg: true,
			want:   false,
		},
		{
			name:  "non-parsable enabled value → active (not silently disabled)",
			goMod: goModWithSQLite(),
			cfg: fakeConfig{values: map[string]any{
				"database.url":                ":memory:",
				"helix.starters.data.enabled": "badvalue",
			}},
			useCfg: true,
			want:   true,
		},
		{
			name:  "override int 0 → inactive",
			goMod: goModWithSQLite(),
			cfg: fakeConfig{values: map[string]any{
				"database.url":                ":memory:",
				"helix.starters.data.enabled": int(0),
			}},
			useCfg: true,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chdirWithGoMod(t, tt.goMod)

			var s *Starter
			if tt.useCfg {
				s = New(tt.cfg)
			} else {
				s = New(nil)
			}

			if got := s.Condition(); got != tt.want {
				t.Fatalf("Condition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConditionMissingGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	chdirForTest(t, tmpDir)

	s := New(fakeConfig{values: map[string]any{"database.url": ":memory:"}})
	if s.Condition() {
		t.Fatal("Condition() = true with missing go.mod, want false")
	}
}

func TestConditionWalkUpDetectsGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	goModContent := `module example.com/app

require gorm.io/driver/sqlite v1.5.4
`
	if err := os.WriteFile(goModPath, []byte(goModContent), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	subDir := filepath.Join(tmpDir, "subdir", "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	chdirForTest(t, subDir)

	s := New(fakeConfig{values: map[string]any{"database.url": ":memory:"}})
	if !s.Condition() {
		t.Fatal("Condition() = false with go.mod in parent, want true")
	}
}

func TestConfigureNilContainerIsNoOp(_ *testing.T) {
	s := New(fakeConfig{values: map[string]any{"database.url": ":memory:"}})
	_ = s.Configure(nil) // must not panic
}

func TestConfigureRegistersLifecycle(t *testing.T) {
	chdirWithGoMod(t, goModWithSQLite())

	cfg := fakeConfig{values: map[string]any{"database.url": ":memory:"}}
	container := newTestContainer()

	if err := New(cfg).Configure(container); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	lifecycles, err := core.ResolveAll[core.Lifecycle](container)
	if err != nil {
		t.Fatalf("ResolveAll[core.Lifecycle] error = %v", err)
	}
	if len(lifecycles) == 0 {
		t.Fatal("Configure did not register any lifecycle")
	}
}

func TestConfigure_PropagatesRegisterComponentError(t *testing.T) {
	chdirWithGoMod(t, goModWithSQLite())

	cfg := fakeConfig{values: map[string]any{"database.url": ":memory:"}}
	container := core.NewContainer()

	err := New(cfg).Configure(container)
	if err == nil {
		t.Fatal("Configure() error = nil, want register error")
	}
	if !errors.Is(err, core.ErrUnresolvable) {
		t.Fatalf("Configure() error = %v, want ErrUnresolvable", err)
	}
	if !strings.Contains(err.Error(), "data starter: register") {
		t.Fatalf("Configure() error = %q, want register context", err.Error())
	}
}

func TestConfigure_IdempotentDoesNotReplaceLifecycle(t *testing.T) {
	chdirWithGoMod(t, goModWithSQLite())

	cfg := fakeConfig{values: map[string]any{"database.url": ":memory:"}}
	container := newTestContainer()
	starter := New(cfg)

	if err := starter.Configure(container); err != nil {
		t.Fatalf("first Configure() error = %v", err)
	}
	first := singleLifecycle(t, container)

	if err := starter.Configure(container); err != nil {
		t.Fatalf("second Configure() error = %v", err)
	}
	second := singleLifecycle(t, container)

	if first != second {
		t.Fatalf("second Configure() replaced lifecycle: first=%p second=%p", first, second)
	}
}

func TestConfigureRegistersDBComponents(t *testing.T) {
	chdirWithGoMod(t, goModWithSQLite())

	cfg := fakeConfig{values: map[string]any{"database.url": ":memory:"}}
	container := newTestContainer()

	if err := New(cfg).Configure(container); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	if err := container.Start(); err != nil {
		t.Fatalf("container.Start() error = %v", err)
	}
	_ = container.Shutdown()
}

func singleLifecycle(t *testing.T, container *core.Container) core.Lifecycle {
	t.Helper()

	lifecycles, err := core.ResolveAll[core.Lifecycle](container)
	if err != nil {
		t.Fatalf("ResolveAll[core.Lifecycle] error = %v", err)
	}
	if len(lifecycles) != 1 {
		t.Fatalf("registered lifecycles = %d, want 1", len(lifecycles))
	}
	return lifecycles[0]
}

func TestContainerStartFailsOnOpenError(t *testing.T) {
	chdirWithGoMod(t, goModWithSQLite())

	container := newTestContainer()

	lc := &databaseLifecycle{startErr: errForTest("forced open error")}
	_ = container.Register(lc)

	if err := container.Start(); err == nil {
		t.Fatal("container.Start() expected error, got nil")
	}
}

func TestLifecycleOnStartWithError(t *testing.T) {
	expected := errForTest("forced")
	lc := &databaseLifecycle{startErr: expected}
	if err := lc.OnStart(); err == nil {
		t.Fatal("OnStart() with startErr expected error, got nil")
	}
}

func TestLifecycleOnStopNilDB(t *testing.T) {
	lc := &databaseLifecycle{}
	if err := lc.OnStop(context.Background()); err != nil {
		t.Fatalf("OnStop() with nil db error = %v", err)
	}
}

func TestAutoMigrateTrueWithModels(t *testing.T) {
	chdirWithGoMod(t, goModWithSQLite())

	type testEntity struct {
		ID uint `gorm:"primaryKey"`
	}

	cfg := fakeConfig{values: map[string]any{
		"database.url":                     ":memory:",
		"helix.starters.data.auto-migrate": true,
	}}
	container := newTestContainer()

	if err := New(cfg, WithAutoMigrateModels(&testEntity{})).Configure(container); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	if err := container.Start(); err != nil {
		t.Fatalf("container.Start() with auto-migrate error = %v", err)
	}
	_ = container.Shutdown()
}

func TestAutoMigrateFalseSkipsMigration(t *testing.T) {
	chdirWithGoMod(t, goModWithSQLite())

	cfg := fakeConfig{values: map[string]any{
		"database.url":                     ":memory:",
		"helix.starters.data.auto-migrate": false,
	}}
	container := newTestContainer()
	if err := New(cfg).Configure(container); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	if err := container.Start(); err != nil {
		t.Fatalf("container.Start() auto-migrate false error = %v", err)
	}
	_ = container.Shutdown()
}

func TestAutoMigrateTrueNoModelsIsNoOp(t *testing.T) {
	chdirWithGoMod(t, goModWithSQLite())

	cfg := fakeConfig{values: map[string]any{
		"database.url":                     ":memory:",
		"helix.starters.data.auto-migrate": true,
	}}
	container := newTestContainer()
	if err := New(cfg).Configure(container); err != nil { // no WithAutoMigrateModels
		t.Fatalf("Configure() error = %v", err)
	}

	if err := container.Start(); err != nil {
		t.Fatalf("container.Start() auto-migrate true no-models error = %v", err)
	}
	_ = container.Shutdown()
}

func TestPoolConfig(t *testing.T) {
	chdirWithGoMod(t, goModWithSQLite())

	cfg := fakeConfig{values: map[string]any{
		"database.url":           ":memory:",
		"database.pool.max-open": 10,
		"database.pool.max-idle": 5,
	}}
	container := newTestContainer()
	if err := New(cfg).Configure(container); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	if err := container.Start(); err != nil {
		t.Fatalf("container.Start() with pool config error = %v", err)
	}
	_ = container.Shutdown()
}

func TestPoolNegativeValueCausesStartError(t *testing.T) {
	chdirWithGoMod(t, goModWithSQLite())

	cfg := fakeConfig{values: map[string]any{
		"database.url":           ":memory:",
		"database.pool.max-open": -1,
	}}
	container := newTestContainer()
	if err := New(cfg).Configure(container); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	if err := container.Start(); err == nil {
		t.Fatal("container.Start() with negative pool value expected error, got nil")
	}
}

func TestPoolNonParsableValueCausesStartError(t *testing.T) {
	chdirWithGoMod(t, goModWithSQLite())

	cfg := fakeConfig{values: map[string]any{
		"database.url":           ":memory:",
		"database.pool.max-open": "bad",
	}}
	container := newTestContainer()
	if err := New(cfg).Configure(container); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	if err := container.Start(); err == nil {
		t.Fatal("container.Start() with non-parsable pool value expected error, got nil")
	}
}

// TestNoGormIOImportsInStarterData is a static guard: fails if starter/data
// directly imports any gorm.io/* path (test imports included).
func TestNoGormIOImportsInStarterData(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("build.ImportDir: %v", err)
	}

	allImports := append(pkg.Imports, pkg.TestImports...)
	for _, imp := range allImports {
		if strings.HasPrefix(imp, "gorm.io/") {
			t.Errorf("starter/data has a direct gorm.io import %q, which is not allowed", imp)
		}
	}
}

// TestLifecycleOnStartPingFailsOnClosedDB verifies the ping-failure branch
// in OnStart() — Open succeeds but the underlying connection is closed before
// Start is called, so Ping returns an error.
func TestLifecycleOnStartPingFailsOnClosedDB(t *testing.T) {
	chdirWithGoMod(t, goModWithSQLite())

	db, err := datagorm.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	lc := &databaseLifecycle{db: db}
	if err := lc.OnStart(); err == nil {
		t.Fatal("OnStart() with closed DB expected error, got nil")
	}
}

// errForTest is a simple error for tests.
type testError string

func errForTest(msg string) error { return testError(msg) }

func (e testError) Error() string { return string(e) }
