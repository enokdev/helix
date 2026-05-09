package scheduling

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

type multiScheduledProvider struct {
	jobs []scheduler.Job
}

func (p *multiScheduledProvider) ScheduledJobs() []scheduler.Job {
	return p.jobs
}

type panicScheduledProvider struct {
	value any
}

func (p *panicScheduledProvider) ScheduledJobs() []scheduler.Job {
	panic(p.value)
}

type recordingScheduler struct {
	mu              sync.Mutex
	jobs            []scheduler.Job
	unregistered    []string
	registerErrName string
	registerErr     error
	unregisterErr   error
}

func (s *recordingScheduler) Register(job scheduler.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job.Name == s.registerErrName {
		return s.registerErr
	}
	s.jobs = append(s.jobs, job)
	return nil
}

func (s *recordingScheduler) Unregister(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unregistered = append(s.unregistered, name)
	if s.unregisterErr != nil {
		return s.unregisterErr
	}
	for i, job := range s.jobs {
		if job.Name == name {
			s.jobs = append(s.jobs[:i], s.jobs[i+1:]...)
			return nil
		}
	}
	return nil
}

func (s *recordingScheduler) Start() {}

func (s *recordingScheduler) Stop(context.Context) {}

func (s *recordingScheduler) OnStart() error {
	s.Start()
	return nil
}

func (s *recordingScheduler) OnStop(_ context.Context) error {
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

func chdirWithGoMod(t *testing.T, contents string) {
	t.Helper()

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(contents), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	chdirForTest(t, tmpDir)
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
	tmpDir := t.TempDir()
	chdirForTest(t, tmpDir)

	if got := New(nil).Condition(); got {
		t.Fatal("Condition() = true, want false (no go.mod)")
	}
}

func TestConditionWalkUpDetectsGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModWithCron()), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	subDir := filepath.Join(tmpDir, "subdir", "nested")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	chdirForTest(t, subDir)

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
	_ = New(nil).Configure(nil)
}

func TestConfigureRegistersLifecycle(t *testing.T) {
	container := newTestContainer()

	if err := New(nil).Configure(container); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

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
		if err := lifecycle.OnStop(context.Background()); err != nil {
			t.Fatalf("OnStop() error = %v, want nil", err)
		}
	}
}

func TestConfigure_PropagatesSchedulerRegisterError(t *testing.T) {
	container := core.NewContainer()

	err := New(nil).Configure(container)
	if err == nil {
		t.Fatal("Configure() error = nil, want register error")
	}
	if !errors.Is(err, core.ErrUnresolvable) {
		t.Fatalf("Configure() error = %v, want ErrUnresolvable", err)
	}
	if !strings.Contains(err.Error(), "scheduling starter: register scheduler") {
		t.Fatalf("Configure() error = %q, want scheduler register context", err.Error())
	}
}

func TestConfigure_IdempotentDoesNotReplaceLifecycles(t *testing.T) {
	container := newTestContainer()
	starter := New(nil)

	if err := starter.Configure(container); err != nil {
		t.Fatalf("first Configure() error = %v", err)
	}
	first, err := core.ResolveAll[core.Lifecycle](container)
	if err != nil {
		t.Fatalf("ResolveAll first error = %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first lifecycle count = %d, want 2", len(first))
	}

	if err := starter.Configure(container); err != nil {
		t.Fatalf("second Configure() error = %v", err)
	}
	second, err := core.ResolveAll[core.Lifecycle](container)
	if err != nil {
		t.Fatalf("ResolveAll second error = %v", err)
	}
	if len(second) != 2 {
		t.Fatalf("second lifecycle count = %d, want 2", len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("second Configure() replaced lifecycle %d: first=%p second=%p", i, first[i], second[i])
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

	if err := New(nil).Configure(container); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

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
	if err := New(nil).Configure(container); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	if err := container.Start(); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	if err := container.Shutdown(); err != nil {
		t.Fatalf("Shutdown() error = %v, want nil", err)
	}
}

func TestScheduledJobRegistrar_LeavesConcurrencyEnforcementToScheduler(t *testing.T) {
	container := newTestContainer()
	sched := &recordingScheduler{}

	if err := container.Register(&testScheduledProvider{
		job: scheduler.Job{
			Name: "non-concurrent-report",
			Expr: "@every 1s",
			Fn:   func() {},
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
	if sched.jobs[0].AllowConcurrent {
		t.Fatal("AllowConcurrent = true, want false preserved for scheduler enforcement")
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

func TestScheduledJobRegistrar_RollsBackRegisteredJobsOnError(t *testing.T) {
	container := newTestContainer()
	sentinel := errors.New("register failed")
	sched := &recordingScheduler{
		registerErrName: "second-job",
		registerErr:     sentinel,
	}

	if err := container.Register(&multiScheduledProvider{jobs: []scheduler.Job{
		{Name: "first-job", Expr: "@every 1s", Fn: func() {}},
		{Name: "second-job", Expr: "invalid cron", Fn: func() {}},
	}}); err != nil {
		t.Fatalf("Register provider error = %v", err)
	}

	registrar := newScheduledJobRegistrar(container, sched)
	err := registrar.OnStart()
	if err == nil {
		t.Fatal("OnStart() error = nil, want register error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("OnStart() error = %v, want sentinel", err)
	}
	if len(sched.jobs) != 0 {
		t.Fatalf("registered jobs after rollback = %d, want 0", len(sched.jobs))
	}
	if len(sched.unregistered) != 1 || sched.unregistered[0] != "first-job" {
		t.Fatalf("unregistered = %v, want [first-job]", sched.unregistered)
	}
}

func TestScheduledJobRegistrar_ReportsRollbackErrorOnProviderPanic(t *testing.T) {
	container := newTestContainer()
	rollbackErr := errors.New("unregister failed")
	sched := &recordingScheduler{unregisterErr: rollbackErr}

	if err := container.Register(&testScheduledProvider{
		job: scheduler.Job{Name: "first-job", Expr: "@every 1s", Fn: func() {}},
	}); err != nil {
		t.Fatalf("Register first provider error = %v", err)
	}
	if err := container.Register(&panicScheduledProvider{value: "boom"}); err != nil {
		t.Fatalf("Register panic provider error = %v", err)
	}

	registrar := newScheduledJobRegistrar(container, sched)
	err := registrar.OnStart()
	if err == nil {
		t.Fatal("OnStart() error = nil, want panic and rollback error")
	}
	if !strings.Contains(err.Error(), "job provider panicked: boom") {
		t.Fatalf("OnStart() error = %v, want provider panic context", err)
	}
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("OnStart() error = %v, want rollback error", err)
	}
	if len(sched.unregistered) != 1 || sched.unregistered[0] != "first-job" {
		t.Fatalf("unregistered = %v, want [first-job]", sched.unregistered)
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
