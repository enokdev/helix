package helix

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/enokdev/helix/core"
)

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
		Components: []any{&shutdownVerifyService{onStop: func() { shutdownCalled = true }}},
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

type unmarkedComponent struct{}

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
