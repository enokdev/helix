package scheduler

import (
	"errors"
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
			errIs:   ErrInvalidCron,
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

func TestLifecycleStartStop(t *testing.T) {
	s := NewScheduler()

	if err := s.OnStart(); err != nil {
		t.Fatalf("OnStart() failed: %v", err)
	}

	if err := s.OnStop(); err != nil {
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

	if err := s.OnStop(); err != nil {
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

	if err := s.OnStop(); err != nil {
		t.Fatalf("OnStop() failed: %v", err)
	}

	if atomic.LoadInt32(&counter) == 0 {
		t.Error("job never executed")
	}
}
