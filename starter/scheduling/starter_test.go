package scheduling

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/scheduler"
)

type fakeConfig struct {
	values map[string]any
}

type testScheduledProvider struct {
	job scheduler.Job
}

func (p *testScheduledProvider) ScheduledJobs() []scheduler.Job {
	return []scheduler.Job{p.job}
}

type recordingScheduler struct {
	mu   sync.Mutex
	jobs []scheduler.Job
}

func (s *recordingScheduler) Register(job scheduler.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, job)
	return nil
}

func (s *recordingScheduler) Start() {}

func (s *recordingScheduler) Stop(context.Context) {}

func (s *recordingScheduler) OnStart() error {
	s.Start()
	return nil
}

func (s *recordingScheduler) OnStop() error {
	return nil
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

func TestConditionWalkUpDetectsGoMod(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}

	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModWithCron()), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	subDir := filepath.Join(tmpDir, "subdir", "nested")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.Chdir(subDir); err != nil {
		t.Fatalf("chdir to subdir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})

	if got := New(nil).Condition(); !got {
		t.Fatal("Condition() = false with go.mod in parent, want true")
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

func TestConfigureNilContainerIsNoop(_ *testing.T) {
	New(nil).Configure(nil)
}

func TestConfigureRegistersLifecycle(t *testing.T) {
	container := newTestContainer()

	New(nil).Configure(container)

	lifecycles, err := core.ResolveAll[core.Lifecycle](container)
	if err != nil {
		t.Fatalf("ResolveAll error = %v", err)
	}
	if len(lifecycles) != 2 {
		t.Fatalf("lifecycle count = %d, want 2", len(lifecycles))
	}
	for _, lifecycle := range lifecycles {
		if err := lifecycle.OnStart(); err != nil {
			t.Fatalf("OnStart() error = %v, want nil", err)
		}
		if err := lifecycle.OnStop(); err != nil {
			t.Fatalf("OnStop() error = %v, want nil", err)
		}
	}
}

func TestStarter_Configure_RegistersScheduledJobProvider(t *testing.T) {
	container := newTestContainer()
	var runs int32

	if err := container.Register(&testScheduledProvider{
		job: scheduler.Job{
			Name: "hourly-report",
			Expr: "@every 1s",
			Fn: func() {
				atomic.AddInt32(&runs, 1)
			},
		},
	}); err != nil {
		t.Fatalf("Register provider error = %v", err)
	}

	New(nil).Configure(container)

	if err := container.Start(); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := container.Shutdown(); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	deadline := time.After(1500 * time.Millisecond)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatal("scheduled job did not run")
		case <-ticker.C:
			if atomic.LoadInt32(&runs) > 0 {
				return
			}
		}
	}
}

func TestStarter_Configure_NoProviders_NoError(t *testing.T) {
	container := newTestContainer()
	New(nil).Configure(container)

	if err := container.Start(); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	if err := container.Shutdown(); err != nil {
		t.Fatalf("Shutdown() error = %v, want nil", err)
	}
}

func TestScheduledJobRegistrar_WrapsJobsWithSkipLockByDefault(t *testing.T) {
	container := newTestContainer()
	sched := &recordingScheduler{}
	started := make(chan struct{})
	release := make(chan struct{})
	var runs int32

	if err := container.Register(&testScheduledProvider{
		job: scheduler.Job{
			Name: "non-concurrent-report",
			Expr: "@every 1s",
			Fn: func() {
				atomic.AddInt32(&runs, 1)
				started <- struct{}{}
				<-release
			},
		},
	}); err != nil {
		t.Fatalf("Register provider error = %v", err)
	}

	registrar := newScheduledJobRegistrar(container, sched)
	if err := registrar.OnStart(); err != nil {
		t.Fatalf("OnStart() error = %v, want nil", err)
	}

	if len(sched.jobs) != 1 {
		t.Fatalf("registered jobs = %d, want 1", len(sched.jobs))
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sched.jobs[0].Fn()
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("job did not start within 1s")
	}

	sched.jobs[0].Fn()
	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(&runs); got != 1 {
		t.Fatalf("runs = %d, want 1", got)
	}
}

func TestScheduledJobRegistrar_AllowsConcurrentWhenOptedIn(t *testing.T) {
	container := newTestContainer()
	sched := &recordingScheduler{}
	blocked := make(chan struct{}, 2)
	release := make(chan struct{})
	var runs int32

	if err := container.Register(&testScheduledProvider{
		job: scheduler.Job{
			Name:            "concurrent-report",
			Expr:            "@every 1s",
			AllowConcurrent: true,
			Fn: func() {
				atomic.AddInt32(&runs, 1)
				blocked <- struct{}{}
				<-release
			},
		},
	}); err != nil {
		t.Fatalf("Register provider error = %v", err)
	}

	registrar := newScheduledJobRegistrar(container, sched)
	if err := registrar.OnStart(); err != nil {
		t.Fatalf("OnStart() error = %v, want nil", err)
	}

	if len(sched.jobs) != 1 {
		t.Fatalf("registered jobs = %d, want 1", len(sched.jobs))
	}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sched.jobs[0].Fn()
		}()
		<-blocked
	}
	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(&runs); got != 2 {
		t.Fatalf("runs = %d, want 2", got)
	}
}

// ─── ConditionFromContainer tests ────────────────────────────────────────────

func TestSchedulingStarter_ConditionFromContainer(t *testing.T) {
	tests := []struct {
		name    string
		cfg     fakeConfig
		setupFn func(c *core.Container)
		want    bool
	}{
		{
			name:    "enabled: false overrides provider",
			cfg:     fakeConfig{values: map[string]any{schedEnabledKey: false}},
			setupFn: func(c *core.Container) { _ = c.Register(&testScheduledProvider{}) },
			want:    false,
		},
		{
			name:    "enabled: true without provider",
			cfg:     fakeConfig{values: map[string]any{schedEnabledKey: true}},
			setupFn: func(_ *core.Container) {},
			want:    true,
		},
		{
			name:    "provider present without config override",
			cfg:     fakeConfig{values: map[string]any{}},
			setupFn: func(c *core.Container) { _ = c.Register(&testScheduledProvider{}) },
			want:    true,
		},
		{
			name:    "no provider no config",
			cfg:     fakeConfig{values: map[string]any{}},
			setupFn: func(_ *core.Container) {},
			want:    false,
		},
		{
			name:    "nil container returns false",
			cfg:     fakeConfig{values: map[string]any{}},
			setupFn: nil,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.cfg)
			var c *core.Container
			if tt.setupFn != nil {
				c = newTestContainer()
				tt.setupFn(c)
			}
			if got := s.ConditionFromContainer(c); got != tt.want {
				t.Fatalf("ConditionFromContainer() = %v, want %v", got, tt.want)
			}
		})
	}
}
