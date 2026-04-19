package helix

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/starter"
)

// --- helpers -----------------------------------------------------------------

// providerStarter registers a dependency into the container so that a
// component with inject:"true" can receive it during registerAppComponents.
type providerStarter struct {
	active bool
	dep    any
}

func (p *providerStarter) Condition() bool { return p.active }
func (p *providerStarter) Configure(c *core.Container) {
	_ = c.Register(p.dep)
}

// --- integration tests -------------------------------------------------------

// TestRunWithStartersConfiguresBeforeComponents verifies that a starter can
// register a dependency that is later resolved by an application component.
func TestRunWithStartersConfiguresBeforeComponents(t *testing.T) {
	t.Parallel()

	dep := &runDependency{}

	err := Run(App{
		Starters: []starter.Entry{
			{Name: "provider", Order: starter.OrderConfig, Starter: &providerStarter{active: true, dep: dep}},
		},
		Components: []any{
			&runLifecycleService{
				started: make(chan string, 1),
				stopped: make(chan string, 1),
			},
		},
		awaitShutdown: func() error { return nil },
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

// TestRunInvalidStarterPreventsAwaitShutdown verifies that a starter
// validation error short-circuits before awaitShutdown is called.
func TestRunInvalidStarterPreventsAwaitShutdown(t *testing.T) {
	t.Parallel()

	awaitCalled := false

	err := Run(App{
		Starters: []starter.Entry{
			{Name: "", Order: starter.OrderWeb, Starter: nil}, // invalid: empty name + nil
		},
		awaitShutdown: func() error {
			awaitCalled = true
			return nil
		},
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if awaitCalled {
		t.Fatal("Run() called awaitShutdown despite invalid starter")
	}
}

// TestRunStarterLoggerReceivesDebugEvents verifies that the App.Logger
// receives "starter evaluated" debug events during Run.
func TestRunStarterLoggerReceivesDebugEvents(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	err := Run(App{
		Logger: logger,
		Starters: []starter.Entry{
			{Name: "testmod", Order: starter.OrderConfig, Starter: &providerStarter{active: false}},
		},
		awaitShutdown: func() error { return nil },
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("expected debug log output in App.Logger, got none")
	}

	var logEntry map[string]any
	if err := json.NewDecoder(&buf).Decode(&logEntry); err != nil {
		t.Fatalf("failed to decode log JSON: %v", err)
	}
	if name, _ := logEntry["starter"].(string); name != "testmod" {
		t.Errorf("log starter = %q, want %q", name, "testmod")
	}
	if _, ok := logEntry["order"]; !ok {
		t.Error("log entry missing field \"order\"")
	}
	if active, _ := logEntry["active"].(bool); active {
		t.Errorf("log active = %v, want false", active)
	}
}

// TestRunWithoutStartersRemainsCompatible confirms that Run(App{}) without
// starters continues to work exactly as before.
func TestRunWithoutStartersRemainsCompatible(t *testing.T) {
	t.Parallel()

	err := Run(App{
		Components:    []any{&markedService{}},
		awaitShutdown: func() error { return nil },
	})
	if err != nil {
		t.Fatalf("Run() regression: error = %v", err)
	}
}

// TestRunWrapsStarterError verifies the error message format.
func TestRunWrapsStarterError(t *testing.T) {
	t.Parallel()

	err := Run(App{
		Starters: []starter.Entry{
			{Name: "bad", Order: starter.Order(99), Starter: &providerStarter{active: true}},
		},
		awaitShutdown: func() error { return nil },
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, starter.ErrInvalidStarter) {
		t.Errorf("Run() error = %v, want chain containing ErrInvalidStarter", err)
	}
}
