package core

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestContainerStartOrdersLifecycleByDependencies(t *testing.T) {
	t.Parallel()

	recorder := newLifecycleEventRecorder()
	resolver := NewReflectResolver()

	if err := resolver.Register(&lifecycleService{recorder: recorder}); err != nil {
		t.Fatalf("Register(service) error = %v", err)
	}
	if err := resolver.Register(&lifecycleDependency{recorder: recorder}); err != nil {
		t.Fatalf("Register(dependency) error = %v", err)
	}
	if err := resolver.Register(ComponentRegistration{
		Component: &lazyLifecycleComponent{recorder: recorder},
		Lazy:      true,
	}); err != nil {
		t.Fatalf("Register(lazy lifecycle) error = %v", err)
	}

	container := NewContainer(WithResolver(resolver))

	if err := container.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if got, want := recorder.events(), []string{"dependency:start", "service:start"}; !equalStringSlices(got, want) {
		t.Fatalf("Start() events = %v, want %v", got, want)
	}
}

func TestContainerStartFailureRollsBackStartedComponents(t *testing.T) {
	t.Parallel()

	recorder := newLifecycleEventRecorder()
	startErr := errors.New("boom")
	resolver := NewReflectResolver()

	if err := resolver.Register(&lifecycleDependency{recorder: recorder}); err != nil {
		t.Fatalf("Register(dependency) error = %v", err)
	}
	if err := resolver.Register(&lifecycleService{recorder: recorder}); err != nil {
		t.Fatalf("Register(service) error = %v", err)
	}
	if err := resolver.Register(&failingLifecycleService{
		recorder: recorder,
		startErr: startErr,
	}); err != nil {
		t.Fatalf("Register(failing service) error = %v", err)
	}

	container := NewContainer(WithResolver(resolver), WithShutdownTimeout(50*time.Millisecond))

	err := container.Start()
	if !errors.Is(err, startErr) {
		t.Fatalf("Start() error = %v, want wrapped %v", err, startErr)
	}

	if got, want := recorder.events(), []string{
		"dependency:start",
		"service:start",
		"failing:start",
		"service:stop",
		"dependency:stop",
	}; !equalStringSlices(got, want) {
		t.Fatalf("Start() events = %v, want %v", got, want)
	}
}

func TestContainerShutdownContinuesAfterStopErrorAndLogsIt(t *testing.T) {
	t.Parallel()

	recorder := newLifecycleEventRecorder()
	stopErr := errors.New("stop failed")
	var logs bytes.Buffer

	logger := slog.New(slog.NewTextHandler(&logs, nil))
	resolver := NewReflectResolver()

	if err := resolver.Register(&lifecycleDependency{recorder: recorder}); err != nil {
		t.Fatalf("Register(dependency) error = %v", err)
	}
	if err := resolver.Register(&lifecycleService{
		recorder: recorder,
		stopErr:  stopErr,
	}); err != nil {
		t.Fatalf("Register(service) error = %v", err)
	}

	container := NewContainer(WithResolver(resolver), WithLogger(logger))

	if err := container.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	err := container.Shutdown()
	if !errors.Is(err, stopErr) {
		t.Fatalf("Shutdown() error = %v, want wrapped %v", err, stopErr)
	}

	if got, want := recorder.events(), []string{
		"dependency:start",
		"service:start",
		"service:stop",
		"dependency:stop",
	}; !equalStringSlices(got, want) {
		t.Fatalf("Shutdown() events = %v, want %v", got, want)
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, "lifecycleService") || !strings.Contains(logOutput, stopErr.Error()) {
		t.Fatalf("Shutdown() logs = %q, want component name and error", logOutput)
	}
}

func TestContainerShutdownUsesConfiguredTimeoutBudget(t *testing.T) {
	t.Parallel()

	recorder := newLifecycleEventRecorder()
	stopBlock := make(chan struct{})
	resolver := NewReflectResolver()

	if err := resolver.Register(&lifecycleDependency{recorder: recorder}); err != nil {
		t.Fatalf("Register(dependency) error = %v", err)
	}
	if err := resolver.Register(&blockingLifecycleService{
		recorder:  recorder,
		stopBlock: stopBlock,
	}); err != nil {
		t.Fatalf("Register(blocking service) error = %v", err)
	}

	container := NewContainer(
		WithResolver(resolver),
		WithLogger(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))),
		WithShutdownTimeout(20*time.Millisecond),
	)

	if err := container.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	startedAt := time.Now()
	err := container.Shutdown()
	elapsed := time.Since(startedAt)
	close(stopBlock)

	if !errors.Is(err, ErrShutdownTimeout) {
		t.Fatalf("Shutdown() error = %v, want wrapped %v", err, ErrShutdownTimeout)
	}
	if elapsed >= 200*time.Millisecond {
		t.Fatalf("Shutdown() took too long: %v", elapsed)
	}

	if got, want := recorder.events(), []string{
		"dependency:start",
		"blocking:start",
		"blocking:stop",
	}; !equalStringSlices(got, want) {
		t.Fatalf("Shutdown() events = %v, want %v", got, want)
	}
}

type lifecycleEventRecorder struct {
	mu      sync.Mutex
	entries []string
}

func newLifecycleEventRecorder() *lifecycleEventRecorder {
	return &lifecycleEventRecorder{}
}

func (r *lifecycleEventRecorder) add(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, event)
}

func (r *lifecycleEventRecorder) events() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.entries...)
}

type lifecycleDependency struct {
	recorder *lifecycleEventRecorder
}

func (c *lifecycleDependency) OnStart() error {
	c.recorder.add("dependency:start")
	return nil
}

func (c *lifecycleDependency) OnStop(_ context.Context) error {
	c.recorder.add("dependency:stop")
	return nil
}

type lifecycleService struct {
	Dependency *lifecycleDependency `inject:"true"`
	recorder   *lifecycleEventRecorder
	stopErr    error
}

func (c *lifecycleService) OnStart() error {
	c.recorder.add("service:start")
	return nil
}

func (c *lifecycleService) OnStop(_ context.Context) error {
	c.recorder.add("service:stop")
	return c.stopErr
}

type failingLifecycleService struct {
	Service  *lifecycleService `inject:"true"`
	recorder *lifecycleEventRecorder
	startErr error
}

func (c *failingLifecycleService) OnStart() error {
	c.recorder.add("failing:start")
	return c.startErr
}

func (c *failingLifecycleService) OnStop(_ context.Context) error {
	c.recorder.add("failing:stop")
	return nil
}

type lazyLifecycleComponent struct {
	recorder *lifecycleEventRecorder
}

func (c *lazyLifecycleComponent) OnStart() error {
	c.recorder.add("lazy:start")
	return nil
}

func (c *lazyLifecycleComponent) OnStop(_ context.Context) error {
	c.recorder.add("lazy:stop")
	return nil
}

type blockingLifecycleService struct {
	Dependency *lifecycleDependency `inject:"true"`
	recorder   *lifecycleEventRecorder
	stopBlock  <-chan struct{}
}

func (c *blockingLifecycleService) OnStart() error {
	c.recorder.add("blocking:start")
	return nil
}

func (c *blockingLifecycleService) OnStop(ctx context.Context) error {
	c.recorder.add("blocking:stop")
	select {
	case <-c.stopBlock:
	case <-ctx.Done():
	}
	return nil
}

func TestContainerStartExcludesPrototypeScopedLifecycle(t *testing.T) {
	t.Parallel()

	recorder := newLifecycleEventRecorder()
	resolver := NewReflectResolver()

	if err := resolver.Register(&lifecycleDependency{recorder: recorder}); err != nil {
		t.Fatalf("Register(dependency) error = %v", err)
	}
	if err := resolver.Register(ComponentRegistration{
		Component: &lazyLifecycleComponent{recorder: recorder},
		Scope:     ScopePrototype,
	}); err != nil {
		t.Fatalf("Register(prototype lifecycle) error = %v", err)
	}

	container := NewContainer(WithResolver(resolver))

	if err := container.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if got, want := recorder.events(), []string{"dependency:start"}; !equalStringSlices(got, want) {
		t.Fatalf("Start() events = %v, want %v (prototype should be excluded)", got, want)
	}
}

func equalStringSlices(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestContainerShutdownContextCancelsOnStop verifies AC1: when a component's OnStop
// exceeds the shutdown budget, the context passed to OnStop is cancelled, allowing
// well-behaved implementations to detect the signal via ctx.Done().
func TestContainerShutdownContextCancelsOnStop(t *testing.T) {
	t.Parallel()

	ctxCancelled := make(chan struct{})
	stopBlock := make(chan struct{})

	lc := &ctxCancelProbeLifecycle{
		block:       stopBlock,
		ctxDoneSeen: ctxCancelled,
	}

	resolver := NewReflectResolver()
	if err := resolver.Register(lc); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	container := NewContainer(
		WithResolver(resolver),
		WithLogger(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))),
		WithShutdownTimeout(20*time.Millisecond),
	)

	if err := container.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Shutdown will timeout after 20ms; the goroutine inside OnStop should
	// observe ctx.Done() and signal ctxCancelled.
	if err := container.Shutdown(); !errors.Is(err, ErrShutdownTimeout) {
		t.Fatalf("Shutdown() error = %v, want %v", err, ErrShutdownTimeout)
	}

	select {
	case <-ctxCancelled:
		// ctx.Done() was observed — goroutine received cancellation signal
	case <-time.After(200 * time.Millisecond):
		t.Fatal("OnStop goroutine did not observe ctx.Done() after timeout")
	}

	close(stopBlock) // let goroutine exit cleanly
}

type ctxCancelProbeLifecycle struct {
	block       <-chan struct{}
	ctxDoneSeen chan<- struct{}
}

func (l *ctxCancelProbeLifecycle) OnStart() error { return nil }
func (l *ctxCancelProbeLifecycle) OnStop(ctx context.Context) error {
	select {
	case <-l.block:
	case <-ctx.Done():
		close(l.ctxDoneSeen)
	}
	return nil
}
