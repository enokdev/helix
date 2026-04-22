package codegen

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScannerDetectsHelixComponents(t *testing.T) {
	dir := t.TempDir()
	writeScanFixture(t, dir, "components.go", `package generated

import h "github.com/enokdev/helix"

type Service struct{}

type UserService struct {
	h.Service
}

type UserController struct {
	h.Controller
}

type LocalOnly struct {
	Service
}
`)

	result, err := NewScanner(dir).Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	if len(result.Components) != 2 {
		t.Fatalf("components = %#v, want 2 Helix components", result.Components)
	}
	assertComponent(t, result.Components, "UserController", ComponentController)
	assertComponent(t, result.Components, "UserService", ComponentService)
	assertNoComponent(t, result.Components, "LocalOnly")
}

func TestScannerIgnoresGeneratedTestVendorTestdataAndBuildConstraints(t *testing.T) {
	dir := t.TempDir()
	writeScanFixture(t, dir, "component.go", scanComponentSource("IncludedService"))
	writeScanFixture(t, dir, "ignored_gen.go", scanComponentSource("GeneratedService"))
	writeScanFixture(t, dir, "ignored_test.go", scanComponentSource("TestService"))
	writeScanFixture(t, dir, "ignored_build.go", `//go:build ignore
// +build ignore

`+scanComponentSource("BuildIgnoredService"))
	writeScanFixture(t, dir, "vendor/example/component.go", scanComponentSource("VendorService"))
	writeScanFixture(t, dir, "testdata/component.go", scanComponentSource("TestdataService"))
	writeScanFixture(t, dir, ".hidden/component.go", scanComponentSource("HiddenService"))

	result, err := NewScanner(dir).Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	if len(result.Components) != 1 {
		t.Fatalf("components = %#v, want only IncludedService", result.Components)
	}
	assertComponent(t, result.Components, "IncludedService", ComponentService)
}

func TestScannerDetectsAliasedHelixComponents(t *testing.T) {
	dir := t.TempDir()
	writeScanFixture(t, dir, "aliased.go", `package generated

import framework "github.com/enokdev/helix"

type UserService struct {
	framework.Service
}

type UserRepository struct {
	framework.Repository
}
`)

	result, err := NewScanner(dir).Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	if len(result.Components) != 2 {
		t.Fatalf("components = %#v, want 2 Helix components", result.Components)
	}
	assertComponent(t, result.Components, "UserService", ComponentService)
	assertComponent(t, result.Components, "UserRepository", ComponentRepository)
}

func TestScannerRejectsLocalMarkerFalsePositive(t *testing.T) {
	dir := t.TempDir()
	writeScanFixture(t, dir, "false_positive.go", `package generated

type Service struct{}
type Controller struct{}

type LocalService struct {
	Service
}

type LocalController struct {
	Controller
}
`)

	result, err := NewScanner(dir).Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	assertNoComponent(t, result.Components, "LocalService")
	assertNoComponent(t, result.Components, "LocalController")
}

func TestScannerHonorsBuildConstraints(t *testing.T) {
	dir := t.TempDir()
	writeScanFixture(t, dir, "included.go", scanComponentSource("IncludedComponent"))
	writeScanFixture(t, dir, "excluded.go", `//go:build ignore
// +build ignore

`+scanComponentSource("ExcludedComponent"))

	result, err := NewScanner(dir).Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	assertComponent(t, result.Components, "IncludedComponent", ComponentService)
	assertNoComponent(t, result.Components, "ExcludedComponent")
}


func TestScannerDetectsDirectives(t *testing.T) {
	dir := t.TempDir()
	writeScanFixture(t, dir, "directives.go", `package generated

import (
	"context"

	h "github.com/enokdev/helix"
)

type User struct{}

type UserController struct {
	h.Controller
}

//helix:route GET /users/search
//helix:guard role:admin,moderator
func (c *UserController) Search() {}

type UserService struct {
	h.Service
}

//helix:transactional
func (s *UserService) Save(ctx context.Context) error { return nil }

//helix:scheduled 0 0 * * *
func (s *UserService) Daily() {}

//helix:scheduled @every 1h
func (s *UserService) Hourly() {}

type UserRepository interface {
	//helix:query auto
	FindByEmail(ctx context.Context, email string) (*User, error)
}
`)

	result, err := NewScanner(dir).Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	assertDirective(t, result.Directives, "route", "GET /users/search", "UserController.Search")
	assertDirective(t, result.Directives, "guard", "role:admin,moderator", "UserController.Search")
	assertDirective(t, result.Directives, "transactional", "", "UserService.Save")
	assertDirective(t, result.Directives, "scheduled", "0 0 * * *", "UserService.Daily")
	assertDirective(t, result.Directives, "scheduled", "@every 1h", "UserService.Hourly")
	assertDirective(t, result.Directives, "query", "auto", "UserRepository.FindByEmail")
}

func TestScannerRejectsMalformedDirectives(t *testing.T) {
	tests := []struct {
		name    string
		comment string
	}{
		{name: "space after slashes", comment: "// helix:route GET /users"},
		{name: "plus directive", comment: "//+helix:route GET /users"},
		{name: "route missing path", comment: "//helix:route GET"},
		{name: "guard missing argument", comment: "//helix:guard"},
		{name: "transactional extra argument", comment: "//helix:transactional readOnly"},
		{name: "unknown directive", comment: "//helix:unknown value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeScanFixture(t, dir, "invalid.go", `package generated

import h "github.com/enokdev/helix"

type UserController struct {
	h.Controller
}

`+tt.comment+`
func (c *UserController) Search() {}
`)

			_, err := NewScanner(dir).Scan(context.Background())
			if err == nil {
				t.Fatal("Scan returned nil error")
			}
			for _, want := range []string{"cli/codegen: scan generated", "invalid.go:", "UserController.Search", "invalid helix directive"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error = %v, want substring %q", err, want)
				}
			}
		})
	}
}

func scanComponentSource(name string) string {
	return `package generated

import "github.com/enokdev/helix"

type ` + name + ` struct {
	helix.Service
}
`
}

func assertComponent(t *testing.T, components []ComponentInfo, name string, kind ComponentKind) {
	t.Helper()

	for _, component := range components {
		if component.TypeName == name {
			if component.Kind != kind {
				t.Fatalf("component %s kind = %s, want %s", name, component.Kind, kind)
			}
			if component.File == "" || component.Line == 0 || component.Package == "" {
				t.Fatalf("component %s has incomplete source info: %#v", name, component)
			}
			return
		}
	}
	t.Fatalf("component %s not found in %#v", name, components)
}

func assertNoComponent(t *testing.T, components []ComponentInfo, name string) {
	t.Helper()

	for _, component := range components {
		if component.TypeName == name {
			t.Fatalf("component %s unexpectedly found in %#v", name, components)
		}
	}
}

func assertDirective(t *testing.T, directives []DirectiveInfo, name, argument, target string) {
	t.Helper()

	for _, directive := range directives {
		if directive.Name == name && directive.Argument == argument && directive.Target == target {
			if directive.File == "" || directive.Line == 0 || directive.Package == "" {
				t.Fatalf("directive %s has incomplete source info: %#v", name, directive)
			}
			return
		}
	}
	t.Fatalf("directive %s %q on %s not found in %#v", name, argument, target, directives)
}

func writeScanFixture(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
