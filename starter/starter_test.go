package starter_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/starter"
)

// --- helpers -----------------------------------------------------------------

type fakeStarter struct {
	condition      bool
	configureErr   error
	conditionPanic any
	configurePanic any
	conditionCalls int
	configureCalls int
}

func (f *fakeStarter) Condition() bool {
	f.conditionCalls++
	if f.conditionPanic != nil {
		panic(f.conditionPanic)
	}
	return f.condition
}

func (f *fakeStarter) Configure(_ *core.Container) error {
	f.configureCalls++
	if f.configurePanic != nil {
		panic(f.configurePanic)
	}
	return f.configureErr
}

// recorderStarter appends its name to seq on Configure to track call order.
type recorderStarter struct {
	seq  *[]string
	name string
}

func (r *recorderStarter) Condition() bool { return true }
func (r *recorderStarter) Configure(_ *core.Container) error {
	*r.seq = append(*r.seq, r.name)
	return nil
}

func newContainer() *core.Container {
	return core.NewContainer(core.WithResolver(core.NewReflectResolver()))
}

// --- canonical order ---------------------------------------------------------

func TestConfigure_CanonicalOrder(t *testing.T) {
	var seq []string
	entries := []starter.Entry{
		{Name: "data", Order: starter.OrderData, Starter: &recorderStarter{seq: &seq, name: "data"}},
		{Name: "config", Order: starter.OrderConfig, Starter: &recorderStarter{seq: &seq, name: "config"}},
		{Name: "web", Order: starter.OrderWeb, Starter: &recorderStarter{seq: &seq, name: "web"}},
	}

	if err := starter.Configure(newContainer(), entries); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"config", "web", "data"}
	if len(seq) != len(want) {
		t.Fatalf("configure sequence = %v, want %v", seq, want)
	}
	for i := range want {
		if seq[i] != want[i] {
			t.Errorf("position %d: got %q, want %q", i, seq[i], want[i])
		}
	}
}

// --- disabled starter --------------------------------------------------------

func TestConfigure_DisabledStarter(t *testing.T) {
	fs := &fakeStarter{condition: false}
	entries := []starter.Entry{
		{Name: "disabled", Order: starter.OrderWeb, Starter: fs},
	}

	if err := starter.Configure(newContainer(), entries); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.conditionCalls != 1 {
		t.Errorf("Condition() called %d time(s), want 1", fs.conditionCalls)
	}
	if fs.configureCalls != 0 {
		t.Errorf("Configure() called %d time(s), want 0", fs.configureCalls)
	}
}

// --- global uniqueness validation -------------------------------------------

func TestConfigure_RejectsDuplicateOrdersBeforeEvaluation(t *testing.T) {
	first := &fakeStarter{condition: true}
	second := &fakeStarter{condition: true}
	entries := []starter.Entry{
		{Name: "first", Order: starter.OrderObservability, Starter: first},
		{Name: "second", Order: starter.OrderObservability, Starter: second},
	}

	err := starter.Configure(newContainer(), entries)
	if err == nil {
		t.Fatal("Configure() error = nil, want duplicate order error")
	}
	if !errors.Is(err, starter.ErrInvalidStarter) {
		t.Fatalf("Configure() error = %v, want ErrInvalidStarter", err)
	}
	if first.conditionCalls != 0 || second.conditionCalls != 0 {
		t.Fatalf("condition calls = %d/%d, want 0/0", first.conditionCalls, second.conditionCalls)
	}
}

func TestConfigure_RejectsDuplicateNamesBeforeEvaluation(t *testing.T) {
	first := &fakeStarter{condition: true}
	second := &fakeStarter{condition: true}
	entries := []starter.Entry{
		{Name: "duplicate", Order: starter.OrderWeb, Starter: first},
		{Name: "duplicate", Order: starter.OrderData, Starter: second},
	}

	err := starter.Configure(newContainer(), entries)
	if err == nil {
		t.Fatal("Configure() error = nil, want duplicate name error")
	}
	if !errors.Is(err, starter.ErrInvalidStarter) {
		t.Fatalf("Configure() error = %v, want ErrInvalidStarter", err)
	}
	if first.conditionCalls != 0 || second.conditionCalls != 0 {
		t.Fatalf("condition calls = %d/%d, want 0/0", first.conditionCalls, second.conditionCalls)
	}
}

// --- validation table-driven ------------------------------------------------

func TestConfigure_ValidationErrors(t *testing.T) {
	validEntry := starter.Entry{Name: "ok", Order: starter.OrderWeb, Starter: &fakeStarter{condition: true}}

	tests := []struct {
		name      string
		container *core.Container
		entries   []starter.Entry
	}{
		{
			name:      "nil container",
			container: nil,
			entries:   []starter.Entry{validEntry},
		},
		{
			name:      "nil starter",
			container: newContainer(),
			entries:   []starter.Entry{{Name: "bad", Order: starter.OrderWeb, Starter: nil}},
		},
		{
			name:      "empty name",
			container: newContainer(),
			entries:   []starter.Entry{{Name: "", Order: starter.OrderWeb, Starter: &fakeStarter{}}},
		},
		{
			name:      "invalid order",
			container: newContainer(),
			entries:   []starter.Entry{{Name: "bad", Order: starter.Order(99), Starter: &fakeStarter{}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := starter.Configure(tc.container, tc.entries)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, starter.ErrInvalidStarter) {
				t.Errorf("error %v does not wrap ErrInvalidStarter", err)
			}
		})
	}
}

func TestConfigure_RecoversConditionPanic(t *testing.T) {
	err := starter.Configure(newContainer(), []starter.Entry{
		{Name: "panic-condition", Order: starter.OrderWeb, Starter: &fakeStarter{conditionPanic: "boom"}},
	})
	if err == nil {
		t.Fatal("Configure() error = nil, want panic recovery error")
	}
	if !errors.Is(err, starter.ErrInvalidStarter) {
		t.Fatalf("Configure() error = %v, want ErrInvalidStarter", err)
	}
	for _, want := range []string{"panic-condition", "condition", "boom"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Configure() error = %q, want to contain %q", err.Error(), want)
		}
	}
}

func TestConfigure_RecoversConfigurePanic(t *testing.T) {
	err := starter.Configure(newContainer(), []starter.Entry{
		{Name: "panic-configure", Order: starter.OrderWeb, Starter: &fakeStarter{condition: true, configurePanic: "boom"}},
	})
	if err == nil {
		t.Fatal("Configure() error = nil, want panic recovery error")
	}
	if !errors.Is(err, starter.ErrInvalidStarter) {
		t.Fatalf("Configure() error = %v, want ErrInvalidStarter", err)
	}
	for _, want := range []string{"panic-configure", "configure", "boom"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Configure() error = %q, want to contain %q", err.Error(), want)
		}
	}
}

// --- debug logging -----------------------------------------------------------

func TestConfigure_DebugLogs(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	fs := &fakeStarter{condition: true}
	entries := []starter.Entry{
		{Name: "mymod", Order: starter.OrderConfig, Starter: fs},
	}

	if err := starter.Configure(newContainer(), entries, starter.WithLogger(logger)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("expected debug log output, got none")
	}

	var logEntry map[string]any
	if err := json.NewDecoder(&buf).Decode(&logEntry); err != nil {
		t.Fatalf("failed to decode log JSON: %v", err)
	}

	if msg, _ := logEntry["msg"].(string); msg != "starter evaluated" {
		t.Errorf("log msg = %q, want %q", msg, "starter evaluated")
	}
	if name, _ := logEntry["starter"].(string); name != "mymod" {
		t.Errorf("log starter = %q, want %q", name, "mymod")
	}
	if order, _ := logEntry["order"].(float64); int(order) != int(starter.OrderConfig) {
		t.Errorf("log order = %v, want %d", order, int(starter.OrderConfig))
	}
	if active, _ := logEntry["active"].(bool); !active {
		t.Errorf("log active = %v, want true", active)
	}
}

func TestConfigure_PropagatesStarterConfigureErrorWithName(t *testing.T) {
	sentinel := errors.New("register failed")
	entries := []starter.Entry{
		{Name: "broken", Order: starter.OrderWeb, Starter: &fakeStarter{condition: true, configureErr: sentinel}},
	}

	err := starter.Configure(newContainer(), entries)
	if err == nil {
		t.Fatal("Configure() error = nil, want sentinel")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("Configure() error = %v, want sentinel", err)
	}
	if got := err.Error(); !strings.Contains(got, `starter: configure "broken"`) {
		t.Fatalf("Configure() error = %q, want starter name context", got)
	}
}

// --- empty entries (no-op) ---------------------------------------------------

func TestConfigure_EmptyEntries(t *testing.T) {
	if err := starter.Configure(newContainer(), nil); err != nil {
		t.Fatalf("unexpected error with nil entries: %v", err)
	}
	if err := starter.Configure(newContainer(), []starter.Entry{}); err != nil {
		t.Fatalf("unexpected error with empty entries: %v", err)
	}
}

// --- MarkerAwareStarter -------------------------------------------------------

// markerAwareFakeStarter implements MarkerAwareStarter for tests.
type markerAwareFakeStarter struct {
	conditionRet                bool
	conditionFromContainerRet   bool
	configureErr                error
	conditionFromContainerPanic any
	conditionCalls              int
	conditionFromContainerCalls int
	configureCalls              int
}

func (m *markerAwareFakeStarter) Condition() bool {
	m.conditionCalls++
	return m.conditionRet
}

func (m *markerAwareFakeStarter) Configure(_ *core.Container) error {
	m.configureCalls++
	return m.configureErr
}

func (m *markerAwareFakeStarter) ConditionFromContainer(_ *core.Container) bool {
	m.conditionFromContainerCalls++
	if m.conditionFromContainerPanic != nil {
		panic(m.conditionFromContainerPanic)
	}
	return m.conditionFromContainerRet
}

// TestConfigure_SkipsMarkerAwareStarters verifies that Configure (pass 1)
// skips starters that implement MarkerAwareStarter entirely.
func TestConfigure_SkipsMarkerAwareStarters(t *testing.T) {
	mas := &markerAwareFakeStarter{conditionRet: true, conditionFromContainerRet: true}
	regular := &fakeStarter{condition: true}

	entries := []starter.Entry{
		{Name: "regular", Order: starter.OrderConfig, Starter: regular},
		{Name: "marker-aware", Order: starter.OrderSecurity, Starter: mas},
	}

	if err := starter.Configure(newContainer(), entries); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mas.conditionCalls != 0 {
		t.Errorf("MarkerAwareStarter.Condition() called %d time(s) in pass 1, want 0", mas.conditionCalls)
	}
	if mas.conditionFromContainerCalls != 0 {
		t.Errorf("MarkerAwareStarter.ConditionFromContainer() called %d time(s) in pass 1, want 0", mas.conditionFromContainerCalls)
	}
	if mas.configureCalls != 0 {
		t.Errorf("MarkerAwareStarter.Configure() called %d time(s) in pass 1, want 0", mas.configureCalls)
	}
	if regular.configureCalls != 1 {
		t.Errorf("regular starter Configure() called %d time(s), want 1", regular.configureCalls)
	}
}

// TestConfigureMarkerAware_OnlyProcessesMarkerAwareStarters verifies that
// ConfigureMarkerAware (pass 2) only processes MarkerAwareStarter entries.
func TestConfigureMarkerAware_OnlyProcessesMarkerAwareStarters(t *testing.T) {
	mas := &markerAwareFakeStarter{conditionFromContainerRet: true}
	regular := &fakeStarter{condition: true}

	entries := []starter.Entry{
		{Name: "regular", Order: starter.OrderConfig, Starter: regular},
		{Name: "marker-aware", Order: starter.OrderSecurity, Starter: mas},
	}

	if err := starter.ConfigureMarkerAware(newContainer(), entries); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if regular.conditionCalls != 0 {
		t.Errorf("regular Condition() called %d time(s) in pass 2, want 0", regular.conditionCalls)
	}
	if regular.configureCalls != 0 {
		t.Errorf("regular Configure() called %d time(s) in pass 2, want 0", regular.configureCalls)
	}
	if mas.conditionFromContainerCalls != 1 {
		t.Errorf("MarkerAwareStarter.ConditionFromContainer() called %d time(s), want 1", mas.conditionFromContainerCalls)
	}
	if mas.configureCalls != 1 {
		t.Errorf("MarkerAwareStarter.Configure() called %d time(s), want 1", mas.configureCalls)
	}
}

// TestConfigureMarkerAware_NilContainerReturnsError verifies that
// ConfigureMarkerAware returns an error when container is nil.
func TestConfigureMarkerAware_NilContainerReturnsError(t *testing.T) {
	mas := &markerAwareFakeStarter{conditionFromContainerRet: true}
	entries := []starter.Entry{
		{Name: "marker-aware", Order: starter.OrderSecurity, Starter: mas},
	}

	err := starter.ConfigureMarkerAware(nil, entries)
	if err == nil {
		t.Fatal("expected error with nil container, got nil")
	}
	if !errors.Is(err, starter.ErrInvalidStarter) {
		t.Errorf("error %v does not wrap ErrInvalidStarter", err)
	}
}

func TestConfigureMarkerAware_RecoversConditionFromContainerPanic(t *testing.T) {
	mas := &markerAwareFakeStarter{conditionFromContainerPanic: "boom"}
	err := starter.ConfigureMarkerAware(newContainer(), []starter.Entry{
		{Name: "panic-marker", Order: starter.OrderSecurity, Starter: mas},
	})
	if err == nil {
		t.Fatal("ConfigureMarkerAware() error = nil, want panic recovery error")
	}
	if !errors.Is(err, starter.ErrInvalidStarter) {
		t.Fatalf("ConfigureMarkerAware() error = %v, want ErrInvalidStarter", err)
	}
	for _, want := range []string{"panic-marker", "condition-from-container", "boom"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ConfigureMarkerAware() error = %q, want to contain %q", err.Error(), want)
		}
	}
}

func TestConfigureMarkerAware_PropagatesStarterConfigureErrorWithName(t *testing.T) {
	sentinel := errors.New("marker register failed")
	mas := &markerAwareFakeStarter{conditionFromContainerRet: true, configureErr: sentinel}
	entries := []starter.Entry{
		{Name: "marker-broken", Order: starter.OrderSecurity, Starter: mas},
	}

	err := starter.ConfigureMarkerAware(newContainer(), entries)
	if err == nil {
		t.Fatal("ConfigureMarkerAware() error = nil, want sentinel")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("ConfigureMarkerAware() error = %v, want sentinel", err)
	}
	if got := err.Error(); !strings.Contains(got, `starter: configure "marker-broken"`) {
		t.Fatalf("ConfigureMarkerAware() error = %q, want starter name context", got)
	}
}

func TestConfigureMarkerAware_RejectsDuplicateOrdersBeforeEvaluation(t *testing.T) {
	t.Parallel()

	first := &markerAwareFakeStarter{conditionFromContainerRet: true}
	second := &markerAwareFakeStarter{conditionFromContainerRet: true}
	entries := []starter.Entry{
		{Name: "first", Order: starter.OrderSecurity, Starter: first},
		{Name: "second", Order: starter.OrderSecurity, Starter: second},
	}

	err := starter.ConfigureMarkerAware(newContainer(), entries)
	if err == nil {
		t.Fatal("ConfigureMarkerAware() error = nil, want duplicate order error")
	}
	if !errors.Is(err, starter.ErrInvalidStarter) {
		t.Fatalf("ConfigureMarkerAware() error = %v, want ErrInvalidStarter", err)
	}
	if first.conditionFromContainerCalls != 0 || second.conditionFromContainerCalls != 0 {
		t.Fatalf("conditionFromContainer calls = %d/%d, want 0/0 (validation must run before evaluation)",
			first.conditionFromContainerCalls, second.conditionFromContainerCalls)
	}
}

func TestConfigureMarkerAware_RejectsDuplicateNamesBeforeEvaluation(t *testing.T) {
	t.Parallel()

	first := &markerAwareFakeStarter{conditionFromContainerRet: true}
	second := &markerAwareFakeStarter{conditionFromContainerRet: true}
	entries := []starter.Entry{
		{Name: "duplicate", Order: starter.OrderSecurity, Starter: first},
		{Name: "duplicate", Order: starter.OrderScheduling, Starter: second},
	}

	err := starter.ConfigureMarkerAware(newContainer(), entries)
	if err == nil {
		t.Fatal("ConfigureMarkerAware() error = nil, want duplicate name error")
	}
	if !errors.Is(err, starter.ErrInvalidStarter) {
		t.Fatalf("ConfigureMarkerAware() error = %v, want ErrInvalidStarter", err)
	}
	if first.conditionFromContainerCalls != 0 || second.conditionFromContainerCalls != 0 {
		t.Fatalf("conditionFromContainer calls = %d/%d, want 0/0 (validation must run before evaluation)",
			first.conditionFromContainerCalls, second.conditionFromContainerCalls)
	}
}

// TestConfigure_LogsReasonField verifies that the log entry contains the
// "reason" field for a regular (non-MarkerAware) starter.
func TestConfigure_LogsReasonField(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	fs := &fakeStarter{condition: true}
	entries := []starter.Entry{
		{Name: "mymod", Order: starter.OrderConfig, Starter: fs},
	}

	if err := starter.Configure(newContainer(), entries, starter.WithLogger(logger)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var logEntry map[string]any
	if err := json.NewDecoder(&buf).Decode(&logEntry); err != nil {
		t.Fatalf("failed to decode log JSON: %v", err)
	}

	reason, ok := logEntry["reason"]
	if !ok {
		t.Fatal("log entry missing \"reason\" field")
	}
	if _, ok := reason.(string); !ok {
		t.Fatalf("log reason is not a string: %T", reason)
	}
}
