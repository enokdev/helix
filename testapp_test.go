package helix

import (
	"testing"

	"github.com/enokdev/helix/core"
)

type rootTestDependency struct {
	Component
	Name string
}

type rootTestService struct {
	Service
	Dependency *rootTestDependency `inject:"true"`
	Port       int                 `value:"server.port"`
}

func TestNewTestAppAndGetBean(t *testing.T) {
	t.Parallel()

	app := NewTestApp(t,
		TestConfigDefaults(map[string]any{"server.port": 5050}),
		TestComponents(
			&rootTestDependency{Name: "root"},
			&rootTestService{},
		),
	)

	service := GetBean[*rootTestService](app)
	if service.Dependency == nil {
		t.Fatal("service.Dependency is nil")
	}
	if service.Dependency.Name != "root" {
		t.Fatalf("service.Dependency.Name = %q, want root", service.Dependency.Name)
	}
	if service.Port != 5050 {
		t.Fatalf("service.Port = %d, want 5050", service.Port)
	}
}

func TestTestContainerOptions(t *testing.T) {
	t.Parallel()

	app := NewTestApp(t,
		TestContainerOptions(core.WithValueLookup(func(key string) (any, bool) {
			return 5151, key == "server.port"
		})),
		TestComponents(&rootTestDependency{}, &rootTestService{}),
	)

	service := GetBean[*rootTestService](app)
	if service.Port != 5151 {
		t.Fatalf("service.Port = %d, want 5151", service.Port)
	}
}
