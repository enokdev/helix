package observability

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/enokdev/helix/core"
	helixweb "github.com/enokdev/helix/web"
)

// ─── fakes ───────────────────────────────────────────────────────────────────

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

type fakeHTTPServer struct{}

func (f *fakeHTTPServer) Start(_ string) error                                    { return nil }
func (f *fakeHTTPServer) Stop(_ context.Context) error                            { return nil }
func (f *fakeHTTPServer) RegisterRoute(_, _ string, _ helixweb.HandlerFunc) error { return nil }
func (f *fakeHTTPServer) ServeHTTP(_ *http.Request) (*http.Response, error)       { return nil, nil }
func (f *fakeHTTPServer) IsGeneratedOnly() bool                                   { return false }

func newTestContainer() *core.Container {
	return core.NewContainer(core.WithResolver(core.NewReflectResolver()))
}

func containerWithServer() *core.Container {
	c := newTestContainer()
	_ = c.Register(&fakeHTTPServer{})
	return c
}

// ─── Condition tests ─────────────────────────────────────────────────────────

func TestCondition(t *testing.T) {
	tests := []struct {
		name string
		cfg  fakeConfig
		want bool
	}{
		{
			name: "enabled: true",
			cfg:  fakeConfig{values: map[string]any{obsEnabledKey: true}},
			want: true,
		},
		{
			name: "enabled: false overrides auto-detect",
			cfg: fakeConfig{values: map[string]any{
				obsEnabledKey:   false,
				"observability": map[string]any{"foo": "bar"},
			}},
			want: false,
		},
		{
			name: "enabled: false, no observability keys",
			cfg:  fakeConfig{values: map[string]any{obsEnabledKey: false}},
			want: false,
		},
		{
			name: "auto-detect via observability.* key",
			cfg:  fakeConfig{values: map[string]any{"observability": map[string]any{"level": "debug"}}},
			want: true,
		},
		{
			name: "no relevant keys",
			cfg:  fakeConfig{values: map[string]any{}},
			want: false,
		},
		{
			name: "non-parsable enabled value falls through to auto-detect (no key)",
			cfg:  fakeConfig{values: map[string]any{obsEnabledKey: "maybe"}},
			want: false,
		},
		{
			name: "non-parsable enabled value, observability key present → active",
			cfg: fakeConfig{values: map[string]any{
				obsEnabledKey:   "maybe",
				"observability": map[string]any{},
			}},
			want: true,
		},
		{
			name: "enabled: int 0 → false",
			cfg:  fakeConfig{values: map[string]any{obsEnabledKey: int(0)}},
			want: false,
		},
		{
			name: "enabled: int 1 → true",
			cfg:  fakeConfig{values: map[string]any{obsEnabledKey: int(1)}},
			want: true,
		},
		{
			name: "enabled: float64 1 → true (YAML numeric)",
			cfg:  fakeConfig{values: map[string]any{obsEnabledKey: float64(1)}},
			want: true,
		},
		{
			name: "enabled: float64 0 → false (YAML numeric)",
			cfg:  fakeConfig{values: map[string]any{obsEnabledKey: float64(0)}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.cfg)
			if got := s.Condition(); got != tt.want {
				t.Fatalf("Condition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConditionNilConfig(t *testing.T) {
	if got := New(nil).Condition(); got {
		t.Fatal("Condition() = true with nil cfg, want false")
	}
}

// ─── Configure nil-safe ───────────────────────────────────────────────────────

func TestConfigureNilContainerIsNoop(_ *testing.T) {
	// Must not panic.
	New(nil).Configure(nil)
}

// ─── Configure without resolvable server ─────────────────────────────────────

func TestConfigureWithoutServerRegistersFailingLifecycle(t *testing.T) {
	container := newTestContainer()

	New(nil).Configure(container)

	lifecycles, err := core.ResolveAll[core.Lifecycle](container)
	if err != nil {
		t.Fatalf("ResolveAll error = %v", err)
	}
	if len(lifecycles) != 1 {
		t.Fatalf("lifecycle count = %d, want 1", len(lifecycles))
	}

	if err := lifecycles[0].OnStart(); err == nil {
		t.Fatal("OnStart() = nil, want error when server not resolvable")
	}
}

// ─── Configure with server ───────────────────────────────────────────────────

func TestConfigureWithServerRegistersLifecycle(t *testing.T) {
	container := containerWithServer()

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
}

// ─── observabilityLifecycle ───────────────────────────────────────────────────

func TestLifecycleOnStopCallsShutdown(t *testing.T) {
	called := false
	lc := &observabilityLifecycle{
		shutdown: func(ctx context.Context) error {
			called = true
			// Verify a deadline is set (timeout context from OnStop).
			if _, ok := ctx.Deadline(); !ok {
				t.Error("OnStop() passed context.Background() without deadline, want timeout context")
			}
			return nil
		},
	}

	if err := lc.OnStop(); err != nil {
		t.Fatalf("OnStop() error = %v, want nil", err)
	}
	if !called {
		t.Fatal("shutdown not called")
	}
}

func TestLifecycleOnStopNilShutdownIsNoop(t *testing.T) {
	lc := &observabilityLifecycle{}
	if err := lc.OnStop(); err != nil {
		t.Fatalf("OnStop() error = %v, want nil", err)
	}
}

func TestLifecycleOnStopWrapsShutdownError(t *testing.T) {
	sentinel := errors.New("otel flush failed")
	lc := &observabilityLifecycle{
		shutdown: func(_ context.Context) error { return sentinel },
	}

	err := lc.OnStop()
	if err == nil {
		t.Fatal("OnStop() = nil, want error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("OnStop() error = %v, want wrapping %v", err, sentinel)
	}
}

func TestLifecycleOnStartReturnsStartErr(t *testing.T) {
	sentinel := errors.New("server not found")
	lc := &observabilityLifecycle{startErr: sentinel}

	err := lc.OnStart()
	if !errors.Is(err, sentinel) {
		t.Fatalf("OnStart() error = %v, want %v", err, sentinel)
	}
}
