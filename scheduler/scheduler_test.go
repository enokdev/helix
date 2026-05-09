package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/enokdev/helix/core"
)

func TestNewScheduler(t *testing.T) {
	s := NewScheduler()
	if s == nil {
		t.Fatal("expected scheduler, got nil")
	}
	if _, ok := s.(core.Lifecycle); !ok {
		t.Fatal("expected scheduler to implement core.Lifecycle")
	}
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name    string
		job     Job
		wantErr bool
		errIs   error
	}{
		{
			name: "Valid job",
			job: Job{
				Name: "test-job",
				Expr: "@every 1s",
				Fn:   func() {},
			},
			wantErr: false,
		},
		{
			name: "Invalid expression",
			job: Job{
				Name: "bad-expr",
				Expr: "invalid cron",
				Fn:   func() {},
			},
			wantErr: true,
			errIs:   ErrInvalidCron,
		},
		{
			name: "Nil function",
			job: Job{
				Name: "nil-fn",
				Expr: "@every 1s",
				Fn:   nil,
			},
			wantErr: true,
			errIs:   ErrInvalidJob,
		},
		{
			name: "Empty name",
			job: Job{
				Name: " ",
				Expr: "@every 1s",
				Fn:   func() {},
			},
			wantErr: true,
			errIs:   ErrInvalidJob,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewScheduler()
			err := s.Register(tt.job)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Register() err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errIs != nil && !errors.Is(err, tt.errIs) {
				t.Errorf("Register() err = %v, expected to wrap %v", err, tt.errIs)
			}
		})
	}
}

func TestRegisterRejectsDuplicateJobName(t *testing.T) {
	s := NewScheduler()
	job := Job{Name: "same-name", Expr: "@every 1s", Fn: func() {}}

	if err := s.Register(job); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}

	err := s.Register(job)
	if err == nil {
		t.Fatal("second Register() error = nil, want duplicate error")
	}
	if !errors.Is(err, ErrDuplicateJob) {
		t.Fatalf("second Register() error = %v, want ErrDuplicateJob", err)
	}
}

func TestUnregisterRejectsSurroundingWhitespace(t *testing.T) {
	s := NewScheduler()
	if err := s.Register(Job{Name: "daily", Expr: "@every 1s", Fn: func() {}}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	unregisterer, ok := s.(interface{ Unregister(string) error })
	if !ok {
		t.Fatal("NewScheduler() does not support Unregister")
	}
	err := unregisterer.Unregister(" daily ")
	if err == nil {
		t.Fatal("Unregister() error = nil, want invalid job error")
	}
	if !errors.Is(err, ErrInvalidJob) {
		t.Fatalf("Unregister() error = %v, want ErrInvalidJob", err)
	}

	err = s.Register(Job{Name: "daily", Expr: "@every 1s", Fn: func() {}})
	if err == nil {
		t.Fatal("Register() after invalid Unregister = nil, want duplicate error")
	}
	if !errors.Is(err, ErrDuplicateJob) {
		t.Fatalf("Register() after invalid Unregister = %v, want ErrDuplicateJob", err)
	}
}

func TestRegisterAppliesSkipLockWhenConcurrencyDisabled(t *testing.T) {
	s := NewScheduler()
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var runs int32

	if err := s.Register(Job{
		Name: "non-concurrent-direct",
		Expr: "@every 1s",
		Fn: func() {
			atomic.AddInt32(&runs, 1)
			started <- struct{}{}
			<-release
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	wrapped := s.(*adapterWrapper).jobFuncForTest("non-concurrent-direct")
	if wrapped == nil {
		t.Fatal("registered job function = nil")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		wrapped()
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first invocation did not start")
	}

	wrapped()
	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(&runs); got != 1 {
		t.Fatalf("runs = %d, want 1", got)
	}
}

func TestRegisterAllowsConcurrentWhenOptedIn(t *testing.T) {
	s := NewScheduler()
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var runs int32

	if err := s.Register(Job{
		Name:            "concurrent-direct",
		Expr:            "@every 1s",
		AllowConcurrent: true,
		Fn: func() {
			atomic.AddInt32(&runs, 1)
			started <- struct{}{}
			<-release
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	wrapped := s.(*adapterWrapper).jobFuncForTest("concurrent-direct")
	if wrapped == nil {
		t.Fatal("registered job function = nil")
	}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wrapped()
		}()
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("invocation did not start")
		}
	}
	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(&runs); got != 2 {
		t.Fatalf("runs = %d, want 2", got)
	}
}

func TestRegisterRecoversAndLogsJobPanic(t *testing.T) {
	var logs bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })

	s := NewScheduler()
	if err := s.Register(Job{
		Name: "panic-job",
		Expr: "@every 1s",
		Fn: func() {
			panic("boom")
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	wrapped := s.(*adapterWrapper).jobFuncForTest("panic-job")
	if wrapped == nil {
		t.Fatal("registered job function = nil")
	}

	wrapped()

	got := logs.String()
	for _, want := range []string{"scheduler: job panic", "job=panic-job", "panic=boom"} {
		if !strings.Contains(got, want) {
			t.Fatalf("logs = %q, want to contain %q", got, want)
		}
	}
}

func TestLifecycleStartStop(t *testing.T) {
	s := NewScheduler()

	if err := s.OnStart(); err != nil {
		t.Fatalf("OnStart() failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.OnStop(ctx); err != nil {
		t.Fatalf("OnStop() failed: %v", err)
	}
}

func TestGracefulShutdown(t *testing.T) {
	s := NewScheduler()

	var counter int32

	// robfig/cron v3 minimum resolution is 1 second; @every Xms rounds up to 1s.
	job := Job{
		Name: "long-job",
		Expr: "@every 1s",
		Fn: func() {
			atomic.AddInt32(&counter, 1)
			time.Sleep(200 * time.Millisecond)
		},
	}

	if err := s.Register(job); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	if err := s.OnStart(); err != nil {
		t.Fatalf("OnStart() failed: %v", err)
	}

	// wait for at least one execution (1s interval + buffer)
	time.Sleep(1200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.OnStop(ctx); err != nil {
		t.Fatalf("OnStop() failed: %v", err)
	}

	if atomic.LoadInt32(&counter) == 0 {
		t.Error("job never ran")
	}
}

func TestJobExecutes(t *testing.T) {
	s := NewScheduler()
	var counter int32

	// robfig/cron v3 minimum resolution is 1 second.
	job := Job{
		Name: "fast-job",
		Expr: "@every 1s",
		Fn:   func() { atomic.AddInt32(&counter, 1) },
	}

	if err := s.Register(job); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	if err := s.OnStart(); err != nil {
		t.Fatalf("OnStart() failed: %v", err)
	}

	time.Sleep(1200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.OnStop(ctx); err != nil {
		t.Fatalf("OnStop() failed: %v", err)
	}

	if atomic.LoadInt32(&counter) == 0 {
		t.Error("job never executed")
	}
}

// TestSchedulerDoubleStopIsIdempotent verifies AC3: calling Stop(ctx) then
// OnStop(ctx) in sequence produces no error or panic. robfig/cron.Stop() is
// idempotent and returns an already-done context on the second call.
func TestSchedulerDoubleStopIsIdempotent(t *testing.T) {
	t.Parallel()

	s := NewScheduler()
	if err := s.OnStart(); err != nil {
		t.Fatalf("OnStart() failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.Stop(ctx)

	if err := s.OnStop(ctx); err != nil {
		t.Fatalf("OnStop() after Stop() returned unexpected error: %v", err)
	}
}
