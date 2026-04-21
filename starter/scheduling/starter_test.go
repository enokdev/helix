package scheduling

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enokdev/helix/core"
)

type fakeConfig struct {
	values map[string]any
}

func (f fakeConfig) Load(any) error { return nil }
func (f fakeConfig) Lookup(key string) (any, bool) {
	v, ok := f.values[key]
	return v, ok
}
func (f fakeConfig) ConfigFileUsed() string      { return "" }
func (f fakeConfig) AllSettings() map[string]any { return f.values }
func (f fakeConfig) ActiveProfiles() []string    { return nil }

func newTestContainer() *core.Container {
	return core.NewContainer(core.WithResolver(core.NewReflectResolver()))
}

func chdirWithGoMod(t *testing.T, contents string) {
	t.Helper()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(contents), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}

func goModWithCron() string {
	return "module example.com/app\n\nrequire github.com/robfig/cron/v3 v3.0.1\n"
}

func goModWithoutCron() string {
	return "module example.com/app\n\nrequire github.com/spf13/viper v1.20.1\n"
}

// ─── Condition tests ──────────────────────────────────────────────────────────

func TestConditionCronPresent(t *testing.T) {
	chdirWithGoMod(t, goModWithCron())

	if got := New(nil).Condition(); !got {
		t.Fatal("Condition() = false, want true (robfig/cron present)")
	}
}

func TestConditionCronAbsent(t *testing.T) {
	chdirWithGoMod(t, goModWithoutCron())

	if got := New(nil).Condition(); got {
		t.Fatal("Condition() = true, want false (robfig/cron absent)")
	}
}

func TestConditionMissingGoMod(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})

	if got := New(nil).Condition(); got {
		t.Fatal("Condition() = true, want false (no go.mod)")
	}
}

func TestConditionOverrideFalseDisablesWhenCronPresent(t *testing.T) {
	chdirWithGoMod(t, goModWithCron())

	cfg := fakeConfig{values: map[string]any{schedEnabledKey: false}}
	if got := New(cfg).Condition(); got {
		t.Fatal("Condition() = true, want false (override enabled: false)")
	}
}

func TestConditionOverrideTrueWhenCronAbsent(t *testing.T) {
	// Even with enabled: true, robfig/cron must be in go.mod to activate.
	chdirWithGoMod(t, goModWithoutCron())

	cfg := fakeConfig{values: map[string]any{schedEnabledKey: true}}
	if got := New(cfg).Condition(); got {
		t.Fatal("Condition() = true, want false (robfig/cron absent, cron check is first)")
	}
}

// ─── Configure tests ──────────────────────────────────────────────────────────

func TestConfigureNilContainerIsNoop(t *testing.T) {
	New(nil).Configure(nil)
}

func TestConfigureRegistersLifecycle(t *testing.T) {
	container := newTestContainer()

	New(nil).Configure(container)

	lifecycles, err := core.ResolveAll[core.Lifecycle](container)
	if err != nil {
		t.Fatalf("ResolveAll error = %v", err)
	}
	if len(lifecycles) != 1 {
		t.Fatalf("lifecycle count = %d, want 1", len(lifecycles))
	}
	if err := lifecycles[0].OnStart(); err != nil {
		t.Fatalf("OnStart() error = %v, want nil", err)
	}
	if err := lifecycles[0].OnStop(); err != nil {
		t.Fatalf("OnStop() error = %v, want nil", err)
	}
}
