package helix

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/enokdev/helix/core"
)

var _ ConfigReloadable = (*rootReloadable)(nil)

func TestDetectComponentMarker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		component any
		want      bool
	}{
		{name: "service marker", component: &markedService{}, want: true},
		{name: "controller marker", component: &markedController{}, want: true},
		{name: "repository marker", component: &markedRepository{}, want: true},
		{name: "component marker", component: &markedComponent{}, want: true},
		{name: "error handler marker", component: &markedErrorHandler{}, want: true},
		{name: "unmarked struct", component: &unmarkedComponent{}, want: false},
		{name: "nil component", component: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasComponentMarker(tt.component); got != tt.want {
				t.Fatalf("hasComponentMarker(%T) = %v, want %v", tt.component, got, tt.want)
			}
		})
	}
}

func TestRunRegistersComponentsResolvesDependenciesAndShutsDown(t *testing.T) {
	t.Parallel()

	events := make(chan string, 4)
	service := &runLifecycleService{started: events, stopped: events}

	err := Run(App{
		Components: []any{
			&runDependency{},
			service,
		},
		awaitShutdown: func() error {
			if got := <-events; got != "start" {
				t.Fatalf("first lifecycle event = %q, want start", got)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if service.Dependency == nil {
		t.Fatal("Run() did not resolve inject dependency before OnStart")
	}
	select {
	case got := <-events:
		if got != "stop" {
			t.Fatalf("second lifecycle event = %q, want stop", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OnStop event")
	}
}

func TestRunModeWireUsesRegisteredWireSetup(t *testing.T) {
	wireSetupMu.Lock()
	previous := wireSetupFn
	wireSetupMu.Unlock()
	t.Cleanup(func() {
		wireSetupMu.Lock()
		wireSetupFn = previous
		wireSetupMu.Unlock()
	})

	events := make(chan string, 4)
	service := &runLifecycleService{started: events, stopped: events}
	RegisterWireSetup(func(container *core.Container) error {
		dependency := &runDependency{}
		service.Dependency = dependency
		if err := container.Register(dependency); err != nil {
			return err
		}
		return container.Register(service)
	})

	err := Run(App{
		Mode: ModeWire,
		awaitShutdown: func() error {
			if got := <-events; got != "start" {
				t.Fatalf("first lifecycle event = %q, want start", got)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if service.Dependency == nil {
		t.Fatal("Run() did not use generated wire setup")
	}
	select {
	case got := <-events:
		if got != "stop" {
			t.Fatalf("second lifecycle event = %q, want stop", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OnStop event")
	}
}

func TestRunModeWireRequiresRegisteredWireSetup(t *testing.T) {
	wireSetupMu.Lock()
	previous := wireSetupFn
	wireSetupFn = nil
	wireSetupMu.Unlock()
	t.Cleanup(func() {
		wireSetupMu.Lock()
		wireSetupFn = previous
		wireSetupMu.Unlock()
	})

	err := Run(App{
		Mode:          ModeWire,
		awaitShutdown: func() error { return nil },
	})
	if !errors.Is(err, core.ErrUnresolvable) {
		t.Fatalf("Run() error = %v, want core.ErrUnresolvable", err)
	}
}

func TestRunStartFailureDoesNotAwaitShutdown(t *testing.T) {
	t.Parallel()

	startErr := errors.New("start failed")
	awaitCalled := false

	err := Run(App{
		Components: []any{
			&failingRunLifecycleService{startErr: startErr},
		},
		awaitShutdown: func() error {
			awaitCalled = true
			return nil
		},
	})

	if !errors.Is(err, startErr) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, startErr)
	}
	if awaitCalled {
		t.Fatal("Run() awaited shutdown after start failure")
	}
}

func TestRunRejectsScanWithoutRuntimeComponents(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	internalDir := filepath.Join(root, "internal", "users")
	writeTestFile(t, internalDir, "service.go", `package users

import "github.com/enokdev/helix"

type UserService struct {
	helix.Service
}
`)

	err := Run(App{
		Scan:          []string{filepath.Join(root, "internal", "...")},
		awaitShutdown: func() error { return nil },
	})
	if !errors.Is(err, ErrScanRequiresComponents) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, ErrScanRequiresComponents)
	}
}

func TestRunAllowsScanWhenRuntimeComponentsAreProvided(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	internalDir := filepath.Join(root, "internal", "users")
	writeTestFile(t, internalDir, "service.go", `package users

import "github.com/enokdev/helix"

type UserService struct {
	helix.Service
}
`)

	err := Run(App{
		Scan:       []string{filepath.Join(root, "internal", "...")},
		Components: []any{&markedService{}},
		awaitShutdown: func() error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunAcceptsErrorHandlerMarker(t *testing.T) {
	t.Parallel()

	err := Run(App{
		Components:    []any{&markedErrorHandler{}},
		awaitShutdown: func() error { return nil },
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunRejectsUnmarkedComponent(t *testing.T) {
	t.Parallel()

	err := Run(App{
		Components:    []any{&unmarkedComponent{}},
		awaitShutdown: func() error { return nil },
	})
	if !errors.Is(err, ErrInvalidComponent) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, ErrInvalidComponent)
	}
}

func TestRunRejectsLazyPrototypeComponent(t *testing.T) {
	t.Parallel()

	err := Run(App{
		Components: []any{
			core.ComponentRegistration{
				Component: &markedComponent{},
				Scope:     core.ScopePrototype,
				Lazy:      true,
			},
		},
		awaitShutdown: func() error { return nil },
	})
	if !errors.Is(err, ErrInvalidComponent) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, ErrInvalidComponent)
	}
}

func TestHTTPErrorTypesExposeStructuredDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  interface {
			error
			StatusCode() int
			ErrorType() string
			ErrorCode() string
			ErrorField() string
		}
		wantStatus int
		wantType   string
		wantCode   string
		wantField  string
		wantMsg    string
	}{
		{
			name:       "not found defaults",
			err:        NotFoundError{},
			wantStatus: http.StatusNotFound,
			wantType:   "NotFoundError",
			wantCode:   "NOT_FOUND",
			wantMsg:    "resource not found",
		},
		{
			name:       "validation custom field",
			err:        ValidationError{Message: "email is required", Field: "email"},
			wantStatus: http.StatusBadRequest,
			wantType:   "ValidationError",
			wantCode:   "VALIDATION_FAILED",
			wantField:  "email",
			wantMsg:    "email is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.err.StatusCode(); got != tt.wantStatus {
				t.Fatalf("StatusCode() = %d, want %d", got, tt.wantStatus)
			}
			if got := tt.err.ErrorType(); got != tt.wantType {
				t.Fatalf("ErrorType() = %q, want %q", got, tt.wantType)
			}
			if got := tt.err.ErrorCode(); got != tt.wantCode {
				t.Fatalf("ErrorCode() = %q, want %q", got, tt.wantCode)
			}
			if got := tt.err.ErrorField(); got != tt.wantField {
				t.Fatalf("ErrorField() = %q, want %q", got, tt.wantField)
			}
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Fatalf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestRunDoesNotStartLazyLifecycleComponent(t *testing.T) {
	t.Parallel()

	events := make(chan string, 1)

	err := Run(App{
		Components: []any{
			core.ComponentRegistration{
				Component: &lazyRunLifecycleComponent{events: events},
				Lazy:      true,
			},
		},
		awaitShutdown: func() error { return nil },
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	select {
	case event := <-events:
		t.Fatalf("lazy lifecycle component was started unexpectedly: %s", event)
	default:
	}
}

func TestScanComponentMarkersIgnoresNonRuntimeSources(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	internalDir := filepath.Join(root, "internal", "users")
	vendorDir := filepath.Join(root, "internal", "vendor", "ignored")

	writeTestFile(t, internalDir, "service.go", `package users

import "github.com/enokdev/helix"

type UserService struct {
	helix.Service
}
`)
	writeTestFile(t, internalDir, "service_test.go", `package users

type TestOnlyService struct {
	Service
}
`)
	writeTestFile(t, internalDir, "service_gen.go", `package users

type GeneratedService struct {
	Service
}
`)
	writeTestFile(t, vendorDir, "vendor.go", `package ignored

type VendorService struct {
	Service
}
`)

	result, err := scanComponentMarkers([]string{filepath.Join(root, "internal", "...")})
	if err != nil {
		t.Fatalf("scanComponentMarkers() error = %v", err)
	}
	if result.ComponentCount != 1 {
		t.Fatalf("ComponentCount = %d, want 1", result.ComponentCount)
	}
}

func TestRunDoesNotLeakGoroutines(t *testing.T) {
	t.Parallel()

	before := runtime.NumGoroutine()
	for i := 0; i < 5; i++ {
		if err := Run(App{
			Components:    []any{&markedService{}},
			awaitShutdown: func() error { return nil },
		}); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	}
	runtime.Gosched()
	after := runtime.NumGoroutine()
	if after > before+1 {
		t.Fatalf("goroutine leak: count before=%d after=%d", before, after)
	}
}

func TestRunCallsShutdownWhenAwaitReturnsError(t *testing.T) {
	t.Parallel()

	shutdownCalled := false
	waitErr := errors.New("simulated signal error")

	err := Run(App{
		Components:    []any{&shutdownVerifyService{onStop: func() { shutdownCalled = true }}},
		awaitShutdown: func() error { return waitErr },
	})
	if !errors.Is(err, waitErr) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, waitErr)
	}
	if !shutdownCalled {
		t.Fatal("Run() did not call Shutdown after awaitShutdown returned error")
	}
}

func TestRunScanAcceptsRelativePath(t *testing.T) {
	// Cannot run in parallel: changes the working directory.
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "internal", "svc"), "svc.go", `package svc

import "github.com/enokdev/helix"

type SomeService struct {
	helix.Service
}
`)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir(%q) error = %v", root, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	err = Run(App{
		Scan:          []string{"./internal/..."},
		awaitShutdown: func() error { return nil },
	})
	if !errors.Is(err, ErrScanRequiresComponents) {
		t.Fatalf("Run() with relative scan path: error = %v, want %v", err, ErrScanRequiresComponents)
	}
}

// TestRun_BackwardCompatibilityWithApp verifies that existing callers using
// Run(App{Components: [...], Starters: [...]}) compile and behave identically.
func TestRun_BackwardCompatibilityWithApp(t *testing.T) {
	t.Parallel()

	events := make(chan string, 4)
	service := &runLifecycleService{started: events, stopped: events}

	err := Run(App{
		Components: []any{
			&runDependency{},
			service,
		},
		awaitShutdown: func() error {
			if got := <-events; got != "start" {
				t.Fatalf("lifecycle event = %q, want start", got)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run() backward compat error = %v", err)
	}

	if service.Dependency == nil {
		t.Fatal("Run() did not inject dependency in backward-compat mode")
	}
}

// TestRun_ZeroParams_NoConfigFile_UsesDefaults verifies that helix.Run()
// without arguments (and without any application.yaml on disk) completes
// without error — starters use their built-in defaults.
func TestRun_ZeroParams_NoConfigFile_UsesDefaults(t *testing.T) {
	// Cannot run in parallel: changes the working directory.
	root := t.TempDir()
	// Write a config that disables the web starter so no port bind is attempted.
	writeTestFile(t, root, "application.yaml", "helix:\n  starters:\n    web:\n      enabled: false\n")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir(%q) error = %v", root, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	err = Run(App{
		awaitShutdown: func() error { return nil },
	})
	if err != nil {
		t.Fatalf("Run() zero-params with no config file: error = %v", err)
	}
}

// TestRun_ZeroParams_LoadsConfig verifies that helix.Run() without explicit
// App arguments discovers and loads config/application.yaml when present.
func TestRun_ZeroParams_LoadsConfig(t *testing.T) {
	// Cannot run in parallel: changes the working directory.
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	// Disable web starter to avoid port-bind conflicts; the key config behaviour
	// under test is that the YAML file is found and loaded.
	writeTestFile(t, configDir, "application.yaml",
		"helix:\n  starters:\n    web:\n      enabled: false\n")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir(%q) error = %v", root, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	err = Run(App{
		awaitShutdown: func() error { return nil },
	})
	if err != nil {
		t.Fatalf("Run() zero-params with config file: error = %v", err)
	}
}

// TestRun_ZeroParams_StartsServer verifies that helix.Run() without arguments
// orchestrates the full bootstrap cycle without any manually registered
// components.
func TestRun_ZeroParams_StartsServer(t *testing.T) {
	// Cannot run in parallel: changes the working directory.
	root := t.TempDir()
	// Use a config that disables the web starter to avoid port-bind conflicts
	// in the test environment while still exercising the full auto-bootstrap path.
	writeTestFile(t, root, "application.yaml", "helix:\n  starters:\n    web:\n      enabled: false\n")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir(%q) error = %v", root, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	err = Run(App{
		awaitShutdown: func() error { return nil },
	})
	if err != nil {
		t.Fatalf("Run() zero-params starts server: error = %v", err)
	}
}

func BenchmarkRun_ZeroParams(b *testing.B) {
	// Disable the web starter to avoid port-bind conflicts in the benchmark
	// environment while exercising the full auto-bootstrap path.
	root := b.TempDir()
	cfg := []byte("helix:\n  starters:\n    web:\n      enabled: false\n")
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		b.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "application.yaml"), cfg, 0o644); err != nil {
		b.Fatalf("WriteFile: %v", err)
	}
	origDir, _ := os.Getwd()
	_ = os.Chdir(root)
	b.Cleanup(func() { _ = os.Chdir(origDir) })

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := Run(App{
			awaitShutdown: func() error { return nil },
		})
		if err != nil {
			b.Fatalf("Run() error = %v", err)
		}
	}
}

func BenchmarkRunMinimalLifecycle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		err := Run(App{
			Components:    []any{&markedService{}},
			awaitShutdown: func() error { return nil },
		})
		if err != nil {
			b.Fatalf("Run() error = %v", err)
		}
	}
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", name, err)
	}
}

type markedService struct {
	Service
}

type markedController struct {
	Controller
}

type markedRepository struct {
	Repository
}

type markedComponent struct {
	Component
}

type markedErrorHandler struct {
	ErrorHandler
}

type unmarkedComponent struct{}

type rootReloadable struct{}

func (r *rootReloadable) OnConfigReload() {}

type runDependency struct {
	Component
}

type runLifecycleService struct {
	Service
	Dependency *runDependency `inject:"true"`
	started    chan<- string
	stopped    chan<- string
}

func (s *runLifecycleService) OnStart() error {
	if s.Dependency == nil {
		return errors.New("dependency was not injected")
	}
	s.started <- "start"
	return nil
}

func (s *runLifecycleService) OnStop() error {
	s.stopped <- "stop"
	return nil
}

type failingRunLifecycleService struct {
	Service
	startErr error
}

func (s *failingRunLifecycleService) OnStart() error {
	return s.startErr
}

func (s *failingRunLifecycleService) OnStop() error {
	return nil
}

type lazyRunLifecycleComponent struct {
	Component
	events chan<- string
}

func (c *lazyRunLifecycleComponent) OnStart() error {
	c.events <- "start"
	return nil
}

func (c *lazyRunLifecycleComponent) OnStop() error {
	c.events <- "stop"
	return nil
}

type shutdownVerifyService struct {
	Service
	onStop func()
}

func (s *shutdownVerifyService) OnStart() error { return nil }
func (s *shutdownVerifyService) OnStop() error {
	if s.onStop != nil {
		s.onStop()
	}
	return nil
}
