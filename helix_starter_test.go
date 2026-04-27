package helix

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/enokdev/helix/core"
	helixsecurity "github.com/enokdev/helix/security"
	"github.com/enokdev/helix/starter"
)

// markerAwareStarter is a test starter that implements MarkerAwareStarter.
// It records whether ConditionFromContainer was called and controls what it returns.
type markerAwareStarter struct {
	conditionFromContainerCalls int
	conditionFromContainerRet   bool
	configureCalls              int
}

func (m *markerAwareStarter) Condition() bool { return false }
func (m *markerAwareStarter) Configure(_ *core.Container) error {
	m.configureCalls++
	return nil
}
func (m *markerAwareStarter) ConditionFromContainer(_ *core.Container) bool {
	m.conditionFromContainerCalls++
	return m.conditionFromContainerRet
}

// securityConfigurerComponent is a test component that satisfies both
// helix.SecurityConfigurer (marker embed) and helixsecurity.Configurer (interface).
type securityConfigurerComponent struct {
	SecurityConfigurer
}

func (s *securityConfigurerComponent) Configure(_ *helixsecurity.HTTPSecurity) {}

// --- helpers -----------------------------------------------------------------

// providerStarter registers a dependency into the container so that a
// component with inject:"true" can receive it during registerAppComponents.
type providerStarter struct {
	active bool
	dep    any
}

func (p *providerStarter) Condition() bool { return p.active }
func (p *providerStarter) Configure(c *core.Container) error {
	_ = c.Register(p.dep)
	return nil
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

// TestRun_MarkerAwareStarterIsCalledAfterRegisterAppComponents verifies that
// ConditionFromContainer is invoked (second pass) after registerAppComponents,
// and not during the first-pass Configure call.
func TestRun_MarkerAwareStarterIsCalledAfterRegisterAppComponents(t *testing.T) {
	t.Parallel()

	mas := &markerAwareStarter{conditionFromContainerRet: true}

	err := Run(App{
		Starters: []starter.Entry{
			{Name: "marker-aware", Order: starter.OrderSecurity, Starter: mas},
		},
		Components:    []any{&markedService{}},
		awaitShutdown: func() error { return nil },
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if mas.conditionFromContainerCalls != 1 {
		t.Errorf("ConditionFromContainer() called %d time(s), want 1", mas.conditionFromContainerCalls)
	}
	if mas.configureCalls != 1 {
		t.Errorf("Configure() called %d time(s), want 1 (ConditionFromContainer returned true)", mas.configureCalls)
	}
}

// TestRun_MarkerAwareStarterNotConfiguredWhenConditionFalse verifies that
// Configure is not called when ConditionFromContainer returns false.
func TestRun_MarkerAwareStarterNotConfiguredWhenConditionFalse(t *testing.T) {
	t.Parallel()

	mas := &markerAwareStarter{conditionFromContainerRet: false}

	err := Run(App{
		Starters: []starter.Entry{
			{Name: "marker-aware", Order: starter.OrderSecurity, Starter: mas},
		},
		Components:    []any{&markedService{}},
		awaitShutdown: func() error { return nil },
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if mas.conditionFromContainerCalls != 1 {
		t.Errorf("ConditionFromContainer() called %d time(s), want 1", mas.conditionFromContainerCalls)
	}
	if mas.configureCalls != 0 {
		t.Errorf("Configure() called %d time(s), want 0 (ConditionFromContainer returned false)", mas.configureCalls)
	}
}

// TestRun_MarkerAwareStarterLogsReason verifies that the logger receives
// "component-marker" as the reason field when a MarkerAwareStarter is activated.
func TestRun_MarkerAwareStarterLogsReason(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mas := &markerAwareStarter{conditionFromContainerRet: true}

	err := Run(App{
		Logger: logger,
		Starters: []starter.Entry{
			{Name: "marker-mod", Order: starter.OrderSecurity, Starter: mas},
		},
		Components:    []any{&markedService{}},
		awaitShutdown: func() error { return nil },
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// The second-pass log entry should contain reason=component-marker.
	decoder := json.NewDecoder(&buf)
	found := false
	for {
		var entry map[string]any
		if err := decoder.Decode(&entry); errors.Is(err, errors.New("EOF")) || err != nil {
			break
		}
		name, _ := entry["starter"].(string)
		reason, _ := entry["reason"].(string)
		if name == "marker-mod" && reason == string(starter.ReasonComponentMarker) {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected log entry with starter=marker-mod reason=%s, got: %s",
			starter.ReasonComponentMarker, buf.String())
	}
}
