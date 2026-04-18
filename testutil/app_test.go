package testutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/enokdev/helix/core"
)

type testAppDependency struct {
	Name string
}

type testAppService struct {
	Dependency *testAppDependency `inject:"true"`
	Port       int                `value:"server.port"`
	Name       string             `value:"app.name"`
}

type testLifecycle struct {
	Started int
	Stopped int
}

func (l *testLifecycle) OnStart() error {
	l.Started++
	return nil
}

func (l *testLifecycle) OnStop() error {
	l.Stopped++
	return nil
}

type failingLifecycle struct {
	stopErr error
}

func (l *failingLifecycle) OnStart() error {
	return nil
}

func (l *failingLifecycle) OnStop() error {
	return l.stopErr
}

func TestNewAppStartsWithoutConfigOrComponents(t *testing.T) {
	t.Parallel()

	app := NewApp(t)
	if app == nil {
		t.Fatal("NewApp() returned nil")
	}
	if app.Container() == nil {
		t.Fatal("Container() returned nil")
	}
	if app.Config() == nil {
		t.Fatal("Config() returned nil")
	}
}

func TestNewAppRegistersComponentsInjectsDependenciesAndValues(t *testing.T) {
	configDir := writeTestConfig(t, "application-test.yaml", `
server:
  port: 8081
app:
  name: test-profile
`)
	t.Setenv("SERVER_PORT", "9090")

	app := NewApp(t,
		WithConfigPaths(configDir),
		WithComponents(
			&testAppDependency{Name: "dependency"},
			&testAppService{},
		),
	)

	service := GetBean[*testAppService](app)
	if service.Dependency == nil {
		t.Fatal("service.Dependency is nil")
	}
	if service.Dependency.Name != "dependency" {
		t.Fatalf("service.Dependency.Name = %q, want dependency", service.Dependency.Name)
	}
	if service.Port != 9090 {
		t.Fatalf("service.Port = %d, want env override 9090", service.Port)
	}
	if service.Name != "test-profile" {
		t.Fatalf("service.Name = %q, want test-profile", service.Name)
	}
}

func TestNewAppMergesBaseAndTestProfileConfiguration(t *testing.T) {
	configDir := writeTestConfig(t, "application.yaml", `
server:
  port: 8080
app:
  name: base
`)
	writeTestConfigInDir(t, configDir, "application-test.yaml", `
app:
  name: test
`)

	app := NewApp(t,
		WithConfigPaths(configDir),
		WithComponents(&testAppDependency{}, &testAppService{}),
	)

	service := GetBean[*testAppService](app)
	if service.Port != 8080 {
		t.Fatalf("service.Port = %d, want base value 8080", service.Port)
	}
	if service.Name != "test" {
		t.Fatalf("service.Name = %q, want test profile override", service.Name)
	}
}

func TestNewAppSupportsConfigDefaults(t *testing.T) {
	t.Parallel()

	app := NewApp(t,
		WithConfigDefaults(map[string]any{
			"server.port": 7070,
			"app.name":    "default-app",
		}),
		WithComponents(&testAppDependency{}, &testAppService{}),
	)

	service := GetBean[*testAppService](app)
	if service.Port != 7070 {
		t.Fatalf("service.Port = %d, want default 7070", service.Port)
	}
	if service.Name != "default-app" {
		t.Fatalf("service.Name = %q, want default-app", service.Name)
	}
}

func TestAppCloseIsIdempotent(t *testing.T) {
	t.Parallel()

	lifecycle := &testLifecycle{}
	app := NewApp(t, WithComponents(lifecycle))

	if lifecycle.Started != 1 {
		t.Fatalf("lifecycle.Started = %d, want 1", lifecycle.Started)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if lifecycle.Stopped != 1 {
		t.Fatalf("lifecycle.Stopped = %d, want 1", lifecycle.Stopped)
	}
}

func TestAppCloseReturnsShutdownError(t *testing.T) {
	t.Parallel()

	stopErr := errors.New("stop failed")
	app := NewApp(t, WithComponents(&failingLifecycle{stopErr: stopErr}))

	err := app.Close()
	if !errors.Is(err, stopErr) {
		t.Fatalf("Close() error = %v, want wrapped %v", err, stopErr)
	}
}

func TestNewAppAppliesContainerOptions(t *testing.T) {
	t.Parallel()

	lookup := func(key string) (any, bool) {
		switch key {
		case "server.port":
			return 6060, true
		case "app.name":
			return "custom", true
		default:
			return nil, false
		}
	}

	app := NewApp(t,
		WithContainerOptions(core.WithValueLookup(lookup)),
		WithComponents(&testAppDependency{}, &testAppService{}),
	)

	service := GetBean[*testAppService](app)
	if service.Port != 6060 {
		t.Fatalf("service.Port = %d, want custom lookup 6060", service.Port)
	}
}

func writeTestConfig(t *testing.T, name, contents string) string {
	t.Helper()

	configDir := filepath.Join(t.TempDir(), "config")
	writeTestConfigInDir(t, configDir, name, contents)
	return configDir
}

func writeTestConfigInDir(t *testing.T, configDir, name, contents string) {
	t.Helper()

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, name), []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
