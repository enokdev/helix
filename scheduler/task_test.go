package scheduler

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWrapError(t *testing.T) {
	tests := []struct {
		name       string
		fn         func() error
		wantLog    bool
		wantFields []string
	}{
		{
			name:    "nil error produces no log",
			fn:      func() error { return nil },
			wantLog: false,
		},
		{
			name:    "non-nil error produces structured log",
			fn:      func() error { return errors.New("boom") },
			wantLog: true,
			wantFields: []string{
				"level=ERROR",
				"msg=\"scheduler: job error\"",
				"job=daily-report",
				"error=boom",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			original := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(original) })

			WrapError("daily-report", tc.fn)()

			got := logs.String()
			if !tc.wantLog {
				if got != "" {
					t.Fatalf("logs = %q, want empty", got)
				}
				return
			}
			for _, want := range tc.wantFields {
				if !strings.Contains(got, want) {
					t.Fatalf("logs = %q, want to contain %q", got, want)
				}
			}
		})
	}
}

func TestWrapSkipIfBusy(t *testing.T) {
	t.Run("skips_concurrent_invocation", func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		var runs int32

		wrapped := WrapSkipIfBusy(func() {
			atomic.AddInt32(&runs, 1)
			started <- struct{}{}
			<-release
		})

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			wrapped()
		}()

		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("job did not start within 1s")
		}

		wrapped()
		close(release)
		wg.Wait()

		if got := atomic.LoadInt32(&runs); got != 1 {
			t.Fatalf("runs = %d, want 1", got)
		}
	})

	t.Run("runs_sequential_invocations", func(t *testing.T) {
		var runs int32
		wrapped := WrapSkipIfBusy(func() {
			atomic.AddInt32(&runs, 1)
		})

		for i := 0; i < 3; i++ {
			wrapped()
		}

		if got := atomic.LoadInt32(&runs); got != 3 {
			t.Fatalf("runs = %d, want 3", got)
		}
	})
}
