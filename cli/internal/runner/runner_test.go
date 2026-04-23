package runner

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestWatchDebouncesReloadAndRestartsChild(t *testing.T) {
	root := newProjectRoot(t)
	w := newFakeWatcher()
	signals := make(chan os.Signal, 1)

	var mu sync.Mutex
	generateCalls := 0
	buildCalls := 0
	startCalls := 0
	var processes []*fakeProcess

	errCh := make(chan error, 1)
	go func() {
		errCh <- Watch(context.Background(), WatchOptions{
			RootDir:  root,
			Debounce: 20 * time.Millisecond,
			signalCh: signals,
			newWatcher: func() (watcher, error) {
				return w, nil
			},
			generate: func(context.Context, string) error {
				mu.Lock()
				generateCalls++
				mu.Unlock()
				return nil
			},
			buildBinary: func(_ context.Context, _ string, outputPath string, _ io.Writer, _ io.Writer) error {
				mu.Lock()
				buildCalls++
				mu.Unlock()
				return os.WriteFile(outputPath, []byte("binary"), 0o755)
			},
			startProcess: func(_ context.Context, _ string, _ []string, _ string, _ io.Writer, _ io.Writer) (process, error) {
				mu.Lock()
				defer mu.Unlock()
				startCalls++
				p := &fakeProcess{}
				processes = append(processes, p)
				return p, nil
			},
		})
	}()

	waitFor(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return startCalls == 1
	})

	w.events <- fsnotify.Event{Name: filepath.Join(root, "app.go"), Op: fsnotify.Write}
	w.events <- fsnotify.Event{Name: filepath.Join(root, "app.go"), Op: fsnotify.Write}

	waitFor(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return startCalls == 2
	})

	signals <- os.Interrupt
	if err := <-errCh; err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if generateCalls != 2 {
		t.Fatalf("generateCalls = %d, want 2", generateCalls)
	}
	if buildCalls != 2 {
		t.Fatalf("buildCalls = %d, want 2", buildCalls)
	}
	if len(processes) != 2 {
		t.Fatalf("len(processes) = %d, want 2", len(processes))
	}
	if len(processes[0].signals) == 0 || processes[0].signals[0] != syscall.SIGTERM {
		t.Fatalf("old process signals = %v, want initial SIGTERM during reload", processes[0].signals)
	}
	if processes[0].waitCalls == 0 {
		t.Fatalf("old process waitCalls = %d, want > 0", processes[0].waitCalls)
	}
	if len(processes[1].signals) == 0 || processes[1].signals[len(processes[1].signals)-1] != os.Interrupt {
		t.Fatalf("new process signals = %v, want final os.Interrupt on shutdown", processes[1].signals)
	}
}

func TestWatchKeepsCurrentProcessWhenGenerateFails(t *testing.T) {
	root := newProjectRoot(t)
	w := newFakeWatcher()
	signals := make(chan os.Signal, 1)

	var mu sync.Mutex
	generateCalls := 0
	buildCalls := 0
	current := &fakeProcess{}

	errCh := make(chan error, 1)
	go func() {
		errCh <- Watch(context.Background(), WatchOptions{
			RootDir:  root,
			Debounce: 20 * time.Millisecond,
			signalCh: signals,
			newWatcher: func() (watcher, error) {
				return w, nil
			},
			generate: func(context.Context, string) error {
				mu.Lock()
				defer mu.Unlock()
				generateCalls++
				if generateCalls == 2 {
					return os.ErrInvalid
				}
				return nil
			},
			buildBinary: func(_ context.Context, _ string, outputPath string, _ io.Writer, _ io.Writer) error {
				mu.Lock()
				buildCalls++
				mu.Unlock()
				return os.WriteFile(outputPath, []byte("binary"), 0o755)
			},
			startProcess: func(_ context.Context, _ string, _ []string, _ string, _ io.Writer, _ io.Writer) (process, error) {
				return current, nil
			},
		})
	}()

	waitFor(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return generateCalls == 1
	})

	w.events <- fsnotify.Event{Name: filepath.Join(root, "app.go"), Op: fsnotify.Write}

	waitFor(t, time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return generateCalls == 2
	})

	if len(current.signals) != 0 {
		t.Fatalf("process.signals = %v, want no restart signal after generate failure", current.signals)
	}
	if current.waitCalls != 0 {
		t.Fatalf("process.waitCalls = %d, want 0 before shutdown", current.waitCalls)
	}
	mu.Lock()
	if buildCalls != 1 {
		mu.Unlock()
		t.Fatalf("buildCalls = %d, want 1", buildCalls)
	}
	mu.Unlock()

	signals <- syscall.SIGTERM
	if err := <-errCh; err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
	if got := current.signals[len(current.signals)-1]; got != syscall.SIGTERM {
		t.Fatalf("final signal = %v, want SIGTERM", got)
	}
}

type fakeWatcher struct {
	events chan fsnotify.Event
	errors chan error
}

func newFakeWatcher() *fakeWatcher {
	return &fakeWatcher{
		events: make(chan fsnotify.Event, 16),
		errors: make(chan error, 1),
	}
}

func (w *fakeWatcher) Add(string) error              { return nil }
func (w *fakeWatcher) Close() error                  { close(w.events); close(w.errors); return nil }
func (w *fakeWatcher) Events() <-chan fsnotify.Event { return w.events }
func (w *fakeWatcher) Errors() <-chan error          { return w.errors }

type fakeProcess struct {
	signals   []os.Signal
	waitCalls int
	killCalls int
}

func (p *fakeProcess) Signal(sig os.Signal) error {
	p.signals = append(p.signals, sig)
	return nil
}

func (p *fakeProcess) Kill() error {
	p.killCalls++
	return nil
}

func (p *fakeProcess) Wait() error {
	p.waitCalls++
	return nil
}

func newProjectRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/runner\n\ngo 1.21.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	return root
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
