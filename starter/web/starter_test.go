package web

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enokdev/helix/core"
	helixweb "github.com/enokdev/helix/web"
)

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

type fakeHTTPServer struct {
	startAddr string
	stopCtx   context.Context
}

func (f *fakeHTTPServer) Start(addr string) error {
	f.startAddr = addr
	return nil
}

func (f *fakeHTTPServer) Stop(ctx context.Context) error {
	f.stopCtx = ctx
	return nil
}

func (f *fakeHTTPServer) RegisterRoute(string, string, helixweb.HandlerFunc) error {
	return nil
}

func (f *fakeHTTPServer) ServeHTTP(*http.Request) (*http.Response, error) {
	return nil, nil
}

func TestStarterCondition(t *testing.T) {
	tests := []struct {
		name       string
		goMod      string
		cfg        fakeConfig
		useConfig  bool
		wantActive bool
	}{
		{
			name:       "fiber present with nil config",
			goMod:      goModWithFiber(),
			wantActive: true,
		},
		{
			name:  "fiber present with config override false",
			goMod: goModWithFiber(),
			cfg: fakeConfig{values: map[string]any{
				"helix.starters.web.enabled": false,
			}},
			useConfig:  true,
			wantActive: false,
		},
		{
			name: "fiber absent",
			goMod: `module example.com/app

require github.com/spf13/viper v1.20.1
`,
			wantActive: false,
		},
		{
			name:  "fiber present with config override true",
			goMod: goModWithFiber(),
			cfg: fakeConfig{values: map[string]any{
				"helix.starters.web.enabled": true,
			}},
			useConfig:  true,
			wantActive: true,
		},
		{
			name:       "fiber present with config but key absent",
			goMod:      goModWithFiber(),
			cfg:        fakeConfig{values: map[string]any{}},
			useConfig:  true,
			wantActive: true,
		},
		{
			name:  "fiber present with config override int 0",
			goMod: goModWithFiber(),
			cfg: fakeConfig{values: map[string]any{
				"helix.starters.web.enabled": int(0),
			}},
			useConfig:  true,
			wantActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chdirWithGoMod(t, tt.goMod)

			var starter *Starter
			if tt.useConfig {
				starter = New(tt.cfg)
			} else {
				starter = New(nil)
			}

			if got := starter.Condition(); got != tt.wantActive {
				t.Fatalf("Condition() = %v, want %v", got, tt.wantActive)
			}
		})
	}
}

func TestStarterConditionMissingGoMod(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	if got := New(nil).Condition(); got {
		t.Fatal("Condition() = true, want false")
	}
}

func TestStarterConfigureRegistersLifecycleWithDefaultPort(t *testing.T) {
	container := newTestContainer()

	New(nil).Configure(container)

	lifecycle := singleLifecycle(t, container)
	serverLifecycle, ok := lifecycle.(*serverLifecycle)
	if !ok {
		t.Fatalf("registered lifecycle type = %T, want *serverLifecycle", lifecycle)
	}
	if serverLifecycle.addr != ":8080" {
		t.Fatalf("addr = %q, want %q", serverLifecycle.addr, ":8080")
	}
}

func TestStarterConfigureRegistersLifecycleWithConfiguredPort(t *testing.T) {
	container := newTestContainer()
	cfg := fakeConfig{values: map[string]any{"server.port": 9090}}

	New(cfg).Configure(container)

	lifecycle := singleLifecycle(t, container)
	serverLifecycle, ok := lifecycle.(*serverLifecycle)
	if !ok {
		t.Fatalf("registered lifecycle type = %T, want *serverLifecycle", lifecycle)
	}
	if serverLifecycle.addr != ":9090" {
		t.Fatalf("addr = %q, want %q", serverLifecycle.addr, ":9090")
	}
}

func TestStarterConfigureRegistersHTTPServer(t *testing.T) {
	container := newTestContainer()

	New(nil).Configure(container)

	var server helixweb.HTTPServer
	if err := container.Resolve(&server); err != nil {
		t.Fatalf("Resolve(HTTPServer) error = %v, want nil", err)
	}
	if server == nil {
		t.Fatal("Resolve(HTTPServer) = nil, want non-nil")
	}
}

func TestServerLifecycleStartStop(t *testing.T) {
	server := &fakeHTTPServer{}
	lifecycle := &serverLifecycle{server: server, addr: ":9090"}

	if err := lifecycle.OnStart(); err != nil {
		t.Fatalf("OnStart() error = %v", err)
	}
	if server.startAddr != ":9090" {
		t.Fatalf("start addr = %q, want %q", server.startAddr, ":9090")
	}

	if err := lifecycle.OnStop(); err != nil {
		t.Fatalf("OnStop() error = %v", err)
	}
	if server.stopCtx == nil {
		t.Fatal("OnStop() did not pass a context to server.Stop")
	}
}

func chdirWithGoMod(t *testing.T, contents string) {
	t.Helper()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(contents), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}

func goModWithFiber() string {
	return "module example.com/app\n\nrequire " +
		strings.Join([]string{"github.com", "gofiber", "fiber", "v2"}, "/") +
		" v2.52.12\n"
}

func newTestContainer() *core.Container {
	return core.NewContainer(core.WithResolver(core.NewReflectResolver()))
}

func singleLifecycle(t *testing.T, container *core.Container) core.Lifecycle {
	t.Helper()

	lifecycles, err := core.ResolveAll[core.Lifecycle](container)
	if err != nil {
		t.Fatalf("ResolveAll[core.Lifecycle]() error = %v", err)
	}
	if len(lifecycles) != 1 {
		t.Fatalf("registered lifecycles = %d, want 1", len(lifecycles))
	}
	return lifecycles[0]
}
