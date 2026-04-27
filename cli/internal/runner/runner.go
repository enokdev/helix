package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/enokdev/helix/cli/internal/codegen"
	"github.com/fsnotify/fsnotify"
)

const defaultDebounce = 150 * time.Millisecond

// WatchOptions configures the hot-reload runner.
type WatchOptions struct {
	RootDir  string
	Args     []string
	Stdout   io.Writer
	Stderr   io.Writer
	Debounce time.Duration

	generate     func(context.Context, string) error
	buildBinary  func(context.Context, string, string, io.Writer, io.Writer) error
	startProcess func(context.Context, string, []string, string, io.Writer, io.Writer) (process, error)
	newWatcher   func() (watcher, error)
	signalCh     <-chan os.Signal
}

type watcher interface {
	Add(string) error
	Close() error
	Events() <-chan fsnotify.Event
	Errors() <-chan error
}

type process interface {
	Signal(os.Signal) error
	Kill() error
	Wait() error
}

type fsWatcher struct {
	*fsnotify.Watcher
}

func (w *fsWatcher) Events() <-chan fsnotify.Event {
	return w.Watcher.Events
}

func (w *fsWatcher) Errors() <-chan error {
	return w.Watcher.Errors
}

type cmdProcess struct {
	cmd *exec.Cmd
}

func (p *cmdProcess) Signal(sig os.Signal) error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Signal(sig)
}

func (p *cmdProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

func (p *cmdProcess) Wait() error {
	return p.cmd.Wait()
}

type runningApp struct {
	process process
	binary  string
}

// Watch starts the application and reloads it whenever a Go file changes.
func Watch(ctx context.Context, opts WatchOptions) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	root, err := projectRoot(opts.RootDir)
	if err != nil {
		return fmt.Errorf("runner: %w", err)
	}
	generate := opts.generate
	if generate == nil {
		generate = defaultGenerate
	}
	buildBinary := opts.buildBinary
	if buildBinary == nil {
		buildBinary = defaultBuildBinary
	}
	startProcess := opts.startProcess
	if startProcess == nil {
		startProcess = defaultStartProcess
	}
	newWatcher := opts.newWatcher
	if newWatcher == nil {
		newWatcher = func() (watcher, error) {
			w, err := fsnotify.NewWatcher()
			if err != nil {
				return nil, err
			}
			return &fsWatcher{Watcher: w}, nil
		}
	}
	debounce := opts.Debounce
	if debounce <= 0 {
		debounce = defaultDebounce
	}
	w, err := newWatcher()
	if err != nil {
		return fmt.Errorf("runner: create watcher: %w", err)
	}
	defer w.Close()
	if err := addWatchTree(w, root); err != nil {
		return fmt.Errorf("runner: watch project: %w", err)
	}

	signalCh := opts.signalCh
	if signalCh == nil {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(ch)
		signalCh = ch
	}

	tmpDir, err := os.MkdirTemp("", "helix-run-*")
	if err != nil {
		return fmt.Errorf("runner: create temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Fprintf(output(opts.Stderr), "runner: warning — cleanup temp dir %s: %v\n", tmpDir, err)
		}
	}()

	current, err := startOrReload(ctx, root, tmpDir, opts.Args, generate, buildBinary, startProcess, opts.Stdout, opts.Stderr, nil)
	if err != nil {
		return err
	}
	defer func() {
		if current.process != nil {
			_ = stopProcess(current.process, syscall.SIGTERM)
		}
	}()

	var timer *time.Timer
	for {
		var timerCh <-chan time.Time
		if timer != nil {
			timerCh = timer.C
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig := <-signalCh:
			if current.process != nil {
				_ = stopProcess(current.process, sig)
				current.process = nil
			}
			return nil
		case err, ok := <-w.Errors():
			if !ok {
				continue
			}
			fmt.Fprintf(output(opts.Stderr), "runner: watcher error: %v\n", err)
		case event, ok := <-w.Events():
			if !ok {
				continue
			}
			if isDirCreateEvent(event) {
				if err := addWatchTree(w, event.Name); err != nil {
					fmt.Fprintf(output(opts.Stderr), "runner: watch new directory %s: %v\n", event.Name, err)
				}
			}
			if !shouldReload(event) {
				continue
			}
			if timer == nil {
				timer = time.NewTimer(debounce)
				continue
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(debounce)
		case <-timerCh:
			timer = nil
			next, err := startOrReload(ctx, root, tmpDir, opts.Args, generate, buildBinary, startProcess, opts.Stdout, opts.Stderr, &current)
			if err != nil {
				fmt.Fprintf(output(opts.Stderr), "%v\n", err)
				continue
			}
			current = next
		}
	}
}

func startOrReload(
	ctx context.Context,
	root, tmpDir string,
	args []string,
	generate func(context.Context, string) error,
	buildBinary func(context.Context, string, string, io.Writer, io.Writer) error,
	startProcess func(context.Context, string, []string, string, io.Writer, io.Writer) (process, error),
	stdout, stderr io.Writer,
	current *runningApp,
) (runningApp, error) {
	// Check context before starting expensive operations
	if err := ctx.Err(); err != nil {
		return runningApp{}, fmt.Errorf("runner: context cancelled before reload: %w", err)
	}
	if err := generate(ctx, root); err != nil {
		return runningApp{}, fmt.Errorf("runner: generate: %w", err)
	}
	// Re-check context after generate, before build
	if err := ctx.Err(); err != nil {
		return runningApp{}, fmt.Errorf("runner: context cancelled during generate: %w", err)
	}
	binary := filepath.Join(tmpDir, fmt.Sprintf("app-%d", time.Now().UnixNano()))
	if err := buildBinary(ctx, root, binary, stdout, stderr); err != nil {
		return runningApp{}, fmt.Errorf("runner: build: %w", err)
	}
	// Re-check context after build, before starting process
	if err := ctx.Err(); err != nil {
		_ = os.Remove(binary)
		return runningApp{}, fmt.Errorf("runner: context cancelled during build: %w", err)
	}
	if current != nil && current.process != nil {
		if err := stopProcess(current.process, syscall.SIGTERM); err != nil {
			return runningApp{}, fmt.Errorf("runner: stop current process: %w", err)
		}
		current.process = nil // nil after successful stop so a subsequent startProcess failure does not leave a dead process reference
		if current.binary != "" {
			_ = os.Remove(current.binary)
		}
	}
	proc, err := startProcess(ctx, binary, args, root, stdout, stderr)
	if err != nil {
		_ = os.Remove(binary)
		return runningApp{}, fmt.Errorf("runner: start app: %w", err)
	}
	return runningApp{process: proc, binary: binary}, nil
}

func defaultGenerate(ctx context.Context, root string) error {
	_, err := codegen.NewGenerator(root).Generate(ctx)
	return err
}

func defaultBuildBinary(ctx context.Context, root, outputPath string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-o", outputPath, "./cmd/...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout = output(stdout)
	cmd.Stderr = output(stderr)
	return cmd.Run()
}

func defaultStartProcess(ctx context.Context, binary string, args []string, root string, stdout, stderr io.Writer) (process, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = root
	cmd.Stdout = output(stdout)
	cmd.Stderr = output(stderr)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	cp := &cmdProcess{cmd: cmd}
	// Background goroutine to reap process immediately after exit (avoid zombie until next reload)
	go func() {
		_ = cmd.Wait()
	}()
	return cp, nil
}

func addWatchTree(w watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && shouldSkipDir(d.Name()) {
			return fs.SkipDir
		}
		return w.Add(path)
	})
}

func shouldSkipDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "bin"
}

func isDirCreateEvent(event fsnotify.Event) bool {
	if event.Op&fsnotify.Create == 0 {
		return false
	}
	info, err := os.Stat(event.Name)
	return err == nil && info.IsDir()
}

func shouldReload(event fsnotify.Event) bool {
	if filepath.Ext(event.Name) != ".go" {
		return false
	}
	return event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0
}

func stopProcess(proc process, sig os.Signal) error {
	if proc == nil {
		return nil
	}
	if err := proc.Signal(sig); err != nil {
		if err := proc.Kill(); err != nil {
			return err
		}
	}
	_ = proc.Wait()
	return nil
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func projectRoot(root string) (string, error) {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	if _, err := os.Stat(filepath.Join(absRoot, "go.mod")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("go.mod not found: %w", err)
		}
		return "", fmt.Errorf("stat go.mod: %w", err)
	}
	return absRoot, nil
}

func output(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
