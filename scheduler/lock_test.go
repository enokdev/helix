package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type recordingLock struct {
	mu          sync.Mutex
	acquiredTTL time.Duration
}

func (r *recordingLock) Acquire(_ context.Context, _ string, ttl time.Duration) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.acquiredTTL = ttl
	return true, nil
}

func (r *recordingLock) Release(_ context.Context, _ string) error { return nil }

func (r *recordingLock) TTL() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.acquiredTTL
}

func TestFakeLock_SingleInstance_AllAcquire(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		lock DistributedLock
	}{
		{name: "noop", lock: &NoOpLock{}},
		{name: "fake", lock: NewFakeLock()},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			acquired, err := tt.lock.Acquire(context.Background(), "job", time.Second)
			if err != nil {
				t.Fatalf("Acquire() error = %v", err)
			}
			if !acquired {
				t.Fatal("Acquire() = false, want true")
			}
		})
	}
}

func TestFakeLock_MultiInstance_OnlyOneExecutes(t *testing.T) {
	t.Parallel()

	lock := NewFakeLock()
	start := make(chan struct{})
	var successCount int32
	var wg sync.WaitGroup

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			acquired, err := lock.Acquire(context.Background(), "shared-job", time.Second)
			if err != nil {
				t.Errorf("Acquire() error = %v", err)
				return
			}
			if acquired {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	close(start)
	wg.Wait()

	if got := atomic.LoadInt32(&successCount); got != 1 {
		t.Fatalf("successful acquisitions = %d, want 1", got)
	}
}

func TestScheduler_WithDistributedLock_SkipsIfNotAcquired(t *testing.T) {
	t.Parallel()

	lock := NewFakeLock()
	acquired, err := lock.Acquire(context.Background(), "locked-job", time.Second)
	if err != nil {
		t.Fatalf("pre-acquire error = %v", err)
	}
	if !acquired {
		t.Fatal("pre-acquire = false, want true")
	}

	s := NewScheduler(WithDistributedLock(lock))
	var runs int32
	if err := s.Register(Job{
		Name: "locked-job",
		Expr: "@every 1s",
		Fn: func() {
			atomic.AddInt32(&runs, 1)
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	wrapped := s.(*adapterWrapper).jobFuncForTest("locked-job")
	if wrapped == nil {
		t.Fatal("registered job function = nil")
	}

	wrapped()

	if got := atomic.LoadInt32(&runs); got != 0 {
		t.Fatalf("runs = %d, want 0", got)
	}
}

func TestScheduler_WithDistributedLockTTL_UsesConfiguredTTL(t *testing.T) {
	t.Parallel()

	lock := &recordingLock{}
	ttl := 5 * time.Minute
	s := NewScheduler(WithDistributedLock(lock), WithDistributedLockTTL(ttl))
	if err := s.Register(Job{
		Name: "ttl-job",
		Expr: "@every 1s",
		Fn:   func() {},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	wrapped := s.(*adapterWrapper).jobFuncForTest("ttl-job")
	if wrapped == nil {
		t.Fatal("registered job function = nil")
	}

	wrapped()

	if got := lock.TTL(); got != ttl {
		t.Fatalf("Acquire() ttl = %s, want %s", got, ttl)
	}
}
