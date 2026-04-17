package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

type reloadableRecorder struct {
	calls chan struct{}
}

func newReloadableRecorder() *reloadableRecorder {
	return &reloadableRecorder{calls: make(chan struct{}, 10)}
}

func (r *reloadableRecorder) OnConfigReload() {
	r.calls <- struct{}{}
}

func (r *reloadableRecorder) callCount() int {
	return len(r.calls)
}

type testSignal string

func (s testSignal) String() string { return string(s) }
func (s testSignal) Signal()        {}

type fakeReloadTicker struct {
	ch      chan time.Time
	stopped chan struct{}
}

func newFakeReloadTicker() *fakeReloadTicker {
	return &fakeReloadTicker{
		ch:      make(chan time.Time, 1),
		stopped: make(chan struct{}),
	}
}

func (t *fakeReloadTicker) C() <-chan time.Time {
	return t.ch
}

func (t *fakeReloadTicker) Stop() {
	select {
	case <-t.stopped:
	default:
		close(t.stopped)
	}
}

func TestReloaderReloadUpdatesResolvedValues(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", `
server:
  port: 8080
app:
  name: helix
`)
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	recorder := newReloadableRecorder()
	reloader, err := NewReloader(loader, &cfg, WithReloadables(recorder))
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	writeConfigFileInDir(t, configDir, "application.yaml", `
server:
  port: 9090
app:
  name: helix-reloaded
`)
	if err := reloader.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Fatalf("cfg.Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.App.Name != "helix-reloaded" {
		t.Fatalf("cfg.App.Name = %q, want helix-reloaded", cfg.App.Name)
	}
	if got, ok := loader.Lookup("server.port"); !ok || fmt.Sprint(got) != "9090" {
		t.Fatalf("Lookup(server.port) = %v, %v; want 9090, true", got, ok)
	}
	settings := loader.AllSettings()
	serverMap, _ := settings["server"].(map[string]any)
	if fmt.Sprint(serverMap["port"]) != "9090" {
		t.Fatalf("AllSettings()[server][port] = %v, want 9090", serverMap["port"])
	}
	if recorder.callCount() != 1 {
		t.Fatalf("OnConfigReload() calls = %d, want 1", recorder.callCount())
	}
}

func TestReloaderReloadPreservesPreviousConfigOnParseError(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", `
server:
  port: 8080
app:
  name: helix
`)
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	recorder := newReloadableRecorder()
	reloader, err := NewReloader(loader, &cfg, WithReloadables(recorder))
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	writeConfigFileInDir(t, configDir, "application.yaml", "server:\n  port: [\n")
	err = reloader.Reload()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Reload() error = %v, want ErrInvalidConfig", err)
	}

	if cfg.Server.Port != 8080 {
		t.Fatalf("cfg.Server.Port = %d, want previous value 8080", cfg.Server.Port)
	}
	if got, ok := loader.Lookup("server.port"); !ok || fmt.Sprint(got) != "8080" {
		t.Fatalf("Lookup(server.port) = %v, %v; want previous 8080, true", got, ok)
	}
	if recorder.callCount() != 0 {
		t.Fatalf("OnConfigReload() calls = %d, want 0", recorder.callCount())
	}
}

func TestReloaderReloadKeepsProfileAndEnvPriority(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", `
server:
  port: 8080
app:
  name: base
  mode: base-mode
`)
	writeConfigFileInDir(t, configDir, "application-dev.yaml", `
server:
  port: 8081
app:
  mode: dev-mode
`)
	t.Setenv(activeProfilesEnv, "dev")
	t.Setenv("SERVER_PORT", "9090")

	loader := NewLoader(WithConfigPaths(configDir))
	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	reloader, err := NewReloader(loader, &cfg)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	writeConfigFileInDir(t, configDir, "application.yaml", `
server:
  port: 7070
app:
  name: base-reloaded
  mode: base-reloaded
`)
	writeConfigFileInDir(t, configDir, "application-dev.yaml", `
server:
  port: 7071
app:
  mode: dev-reloaded
`)
	if err := reloader.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Fatalf("cfg.Server.Port = %d, want ENV override 9090", cfg.Server.Port)
	}
	if cfg.App.Name != "base-reloaded" {
		t.Fatalf("cfg.App.Name = %q, want base-reloaded", cfg.App.Name)
	}
	if cfg.App.Mode != "dev-reloaded" {
		t.Fatalf("cfg.App.Mode = %q, want profile override dev-reloaded", cfg.App.Mode)
	}
	if got, want := loader.ActiveProfiles(), []string{"dev"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ActiveProfiles() = %#v, want %#v", got, want)
	}
}

func TestReloaderStartHandlesSignalFromInjectedSource(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", "server:\n  port: 8080\n")
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	signals := make(chan os.Signal, 1)
	recorder := newReloadableRecorder()
	reloader, err := NewReloader(
		loader,
		&cfg,
		WithReloadables(recorder),
		withReloadSignalSource(func(context.Context) (<-chan os.Signal, func(), error) {
			return signals, func() {}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errs := make(chan error, 1)
	go func() {
		errs <- reloader.Start(ctx)
	}()

	writeConfigFileInDir(t, configDir, "application.yaml", "server:\n  port: 9090\n")
	signals <- testSignal("SIGHUP")

	waitForReloadCall(t, recorder)
	if cfg.Server.Port != 9090 {
		t.Fatalf("cfg.Server.Port = %d, want 9090", cfg.Server.Port)
	}

	cancel()
	if err := <-errs; err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}

func TestReloaderStartPollsAtConfiguredInterval(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", `
server:
  port: 8080
helix:
  config:
    reload-interval: 30s
`)
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	ticker := newFakeReloadTicker()
	var intervalMu sync.Mutex
	var gotInterval time.Duration
	recorder := newReloadableRecorder()
	reloader, err := NewReloader(
		loader,
		&cfg,
		WithReloadables(recorder),
		withReloadSignalSource(func(context.Context) (<-chan os.Signal, func(), error) {
			return nil, func() {}, nil
		}),
		withReloadTickerFactory(func(interval time.Duration) reloadTicker {
			intervalMu.Lock()
			gotInterval = interval
			intervalMu.Unlock()
			return ticker
		}),
	)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errs := make(chan error, 1)
	go func() {
		errs <- reloader.Start(ctx)
	}()

	waitForCondition(t, func() bool {
		intervalMu.Lock()
		defer intervalMu.Unlock()
		return gotInterval == 30*time.Second
	}, "ticker interval to be resolved")
	writeConfigFileInDir(t, configDir, "application.yaml", `
server:
  port: 9090
helix:
  config:
    reload-interval: 30s
`)
	ticker.ch <- time.Now()

	waitForReloadCall(t, recorder)
	if cfg.Server.Port != 9090 {
		t.Fatalf("cfg.Server.Port = %d, want 9090", cfg.Server.Port)
	}

	cancel()
	if err := <-errs; err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	select {
	case <-ticker.stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("ticker was not stopped")
	}
}

func TestReloaderStartLogsReloadErrorsAndContinues(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", "server:\n  port: 8080\n")
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	ticker := newFakeReloadTicker()
	logs := &safeBuffer{}
	recorder := newReloadableRecorder()
	reloader, err := NewReloader(
		loader,
		&cfg,
		WithReloadables(recorder),
		WithReloadLogger(slog.New(slog.NewTextHandler(logs, nil))),
		WithReloadInterval(time.Second),
		withReloadSignalSource(func(context.Context) (<-chan os.Signal, func(), error) {
			return nil, func() {}, nil
		}),
		withReloadTickerFactory(func(time.Duration) reloadTicker {
			return ticker
		}),
	)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errs := make(chan error, 1)
	go func() {
		errs <- reloader.Start(ctx)
	}()

	writeConfigFileInDir(t, configDir, "application.yaml", "server:\n  port: [\n")
	ticker.ch <- time.Now()
	waitForCondition(t, func() bool {
		return logs.String() != ""
	}, "reload error to be logged")

	if cfg.Server.Port != 8080 {
		t.Fatalf("cfg.Server.Port = %d, want previous value 8080", cfg.Server.Port)
	}
	if recorder.callCount() != 0 {
		t.Fatalf("OnConfigReload() calls = %d, want 0", recorder.callCount())
	}

	writeConfigFileInDir(t, configDir, "application.yaml", "server:\n  port: 9090\n")
	ticker.ch <- time.Now()
	waitForReloadCall(t, recorder)
	if cfg.Server.Port != 9090 {
		t.Fatalf("cfg.Server.Port = %d, want 9090 after recovery", cfg.Server.Port)
	}

	cancel()
	if err := <-errs; err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}

func TestLoaderLoadAloneDoesNotStartReload(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", "server:\n  port: 8080\n")
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	writeConfigFileInDir(t, configDir, "application.yaml", "server:\n  port: 9090\n")
	time.Sleep(25 * time.Millisecond)

	if cfg.Server.Port != 8080 {
		t.Fatalf("cfg.Server.Port = %d, want unchanged 8080 without explicit reloader", cfg.Server.Port)
	}
}

func waitForReloadCall(t *testing.T, recorder *reloadableRecorder) {
	t.Helper()

	select {
	case <-recorder.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OnConfigReload")
	}
}

func waitForCondition(t *testing.T, condition func() bool, description string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", description)
}

type safeBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

func TestReloaderNotifiesAllReloadablesInOrder(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", "server:\n  port: 8080\n")
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	var orderMu sync.Mutex
	var order []int
	makeRecorder := func(id int) Reloadable {
		return &funcReloadable{fn: func() {
			orderMu.Lock()
			order = append(order, id)
			orderMu.Unlock()
		}}
	}

	reloader, err := NewReloader(loader, &cfg,
		WithReloadables(makeRecorder(1), makeRecorder(2), makeRecorder(3)),
	)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	writeConfigFileInDir(t, configDir, "application.yaml", "server:\n  port: 9090\n")
	if err := reloader.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	orderMu.Lock()
	got := append([]int(nil), order...)
	orderMu.Unlock()

	if !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("reloadable call order = %v, want [1 2 3]", got)
	}
}

type funcReloadable struct{ fn func() }

func (f *funcReloadable) OnConfigReload() { f.fn() }

func TestNewReloaderValidatesInputs(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", "server:\n  port: 8080\n")
	loader := NewLoader(WithConfigPaths(configDir))
	var cfg loaderTestConfig

	tests := []struct {
		name   string
		loader Loader
		target any
		opts   []ReloadOption
	}{
		{name: "nil loader", loader: nil, target: &cfg},
		{name: "invalid target", loader: loader, target: cfg},
		{name: "nil logger", loader: loader, target: &cfg, opts: []ReloadOption{WithReloadLogger(nil)}},
		{name: "nil reloadable", loader: loader, target: &cfg, opts: []ReloadOption{WithReloadables(nil)}},
		{name: "typed nil reloadable", loader: loader, target: &cfg, opts: []ReloadOption{WithReloadables((*reloadableRecorder)(nil))}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewReloader(tt.loader, tt.target, tt.opts...)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("NewReloader() error = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestReloaderRejectsIntegerConfiguredInterval(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", `
server:
  port: 8080
helix:
  config:
    reload-interval: 30
`)
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	reloader, err := NewReloader(loader, &cfg)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	err = reloader.Start(context.Background())
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Start() error = %v, want ErrInvalidConfig for integer interval", err)
	}
}

func TestReloaderReloadPreservesPreviousConfigOnDecodeError(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", "server:\n  port: 8080\n")
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	recorder := newReloadableRecorder()
	reloader, err := NewReloader(loader, &cfg, WithReloadables(recorder))
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	writeConfigFileInDir(t, configDir, "application.yaml", "server:\n  port: not-an-int\n")
	err = reloader.Reload()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Reload() error = %v, want ErrInvalidConfig", err)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("cfg.Server.Port = %d, want previous value 8080", cfg.Server.Port)
	}
	if got, ok := loader.Lookup("server.port"); !ok || fmt.Sprint(got) != "8080" {
		t.Fatalf("Lookup(server.port) = %v, %v; want previous 8080, true", got, ok)
	}
	if recorder.callCount() != 0 {
		t.Fatalf("OnConfigReload() calls = %d, want 0", recorder.callCount())
	}
}

func TestReloaderEnvOnlyReloadIntervalStartsPolling(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", "server:\n  port: 8080\n")
	t.Setenv("HELIX_CONFIG_RELOAD_INTERVAL", "30s")
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	ticker := newFakeReloadTicker()
	var intervalMu sync.Mutex
	var gotInterval time.Duration
	reloader, err := NewReloader(
		loader,
		&cfg,
		withReloadSignalSource(func(context.Context) (<-chan os.Signal, func(), error) {
			return nil, func() {}, nil
		}),
		withReloadTickerFactory(func(interval time.Duration) reloadTicker {
			intervalMu.Lock()
			gotInterval = interval
			intervalMu.Unlock()
			return ticker
		}),
	)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 1)
	go func() {
		errs <- reloader.Start(ctx)
	}()

	waitForCondition(t, func() bool {
		intervalMu.Lock()
		defer intervalMu.Unlock()
		return gotInterval == 30*time.Second
	}, "env-only reload interval")
	cancel()
	if err := <-errs; err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}

func TestReloaderExplicitIntervalOverridesConfigInterval(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", `
server:
  port: 8080
helix:
  config:
    reload-interval: 30s
`)
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	ticker := newFakeReloadTicker()
	var intervalMu sync.Mutex
	var gotInterval time.Duration
	reloader, err := NewReloader(
		loader,
		&cfg,
		WithReloadInterval(time.Second),
		withReloadSignalSource(func(context.Context) (<-chan os.Signal, func(), error) {
			return nil, func() {}, nil
		}),
		withReloadTickerFactory(func(interval time.Duration) reloadTicker {
			intervalMu.Lock()
			gotInterval = interval
			intervalMu.Unlock()
			return ticker
		}),
	)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 1)
	go func() {
		errs <- reloader.Start(ctx)
	}()

	waitForCondition(t, func() bool {
		intervalMu.Lock()
		defer intervalMu.Unlock()
		return gotInterval == time.Second
	}, "explicit reload interval")
	cancel()
	if err := <-errs; err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}

func TestReloaderRejectsInvalidConfiguredInterval(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", `
server:
  port: 8080
helix:
  config:
    reload-interval: not-a-duration
`)
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	reloader, err := NewReloader(loader, &cfg)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}

	err = reloader.Start(context.Background())
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Start() error = %v, want ErrInvalidConfig", err)
	}
}

func TestReloaderConfigFileUsedRemainsBaseAfterReload(t *testing.T) {
	configDir := writeConfigFile(t, "application.yaml", "server:\n  port: 8080\n")
	loader := NewLoader(WithConfigPaths(configDir))

	var cfg loaderTestConfig
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	reloader, err := NewReloader(loader, &cfg)
	if err != nil {
		t.Fatalf("NewReloader() error = %v", err)
	}
	if err := reloader.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	if got, want := loader.ConfigFileUsed(), filepath.Join(configDir, "application.yaml"); got != want {
		t.Fatalf("ConfigFileUsed() = %q, want %q", got, want)
	}
}
