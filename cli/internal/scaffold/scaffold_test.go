package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewAppCreatesBuildableProject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoRoot := findRepoRoot(t)

	if err := NewApp(Options{
		RootDir:          root,
		Name:             "my-service",
		HelixReplacePath: repoRoot,
	}); err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	appDir := filepath.Join(root, "my-service")
	for _, name := range []string{"go.mod", "main.go", filepath.Join("config", "application.yaml")} {
		if _, err := os.Stat(filepath.Join(appDir, name)); err != nil {
			t.Fatalf("generated file %s stat error = %v", name, err)
		}
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = appDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./... error = %v\n%s", err, output)
	}
}

func TestNewAppRefusesExistingNonEmptyDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	appDir := filepath.Join(root, "my-service")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "README.md"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	err := NewApp(Options{RootDir: root, Name: "my-service"})
	if err == nil {
		t.Fatal("NewApp() error = nil, want existing directory error")
	}
	if !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("NewApp() error = %q, want not empty", err)
	}
}

func TestNewAppRejectsInvalidNames(t *testing.T) {
	t.Parallel()

	tests := []string{"", ".", "..", "/abs", "nested/app", "nested\\app", "Bad_Name", "-weird", "--flag", "1app", "my-app-"}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			err := NewApp(Options{RootDir: t.TempDir(), Name: tt})
			if err == nil {
				t.Fatalf("NewApp(%q) error = nil, want invalid name error", tt)
			}
		})
	}
}

func TestGenerateModuleCreatesUsersPackage(t *testing.T) {
	t.Parallel()

	root := newGoModuleFixture(t)

	if err := GenerateModule(ModuleOptions{RootDir: root, Name: "user"}); err != nil {
		t.Fatalf("GenerateModule() error = %v", err)
	}

	for _, name := range []string{
		filepath.Join("users", "repository.go"),
		filepath.Join("users", "service.go"),
		filepath.Join("users", "controller.go"),
	} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("generated file %s stat error = %v", name, err)
		}
	}

	service := readFile(t, filepath.Join(root, "users", "service.go"))
	if !strings.Contains(service, "package users") ||
		!strings.Contains(service, "helix.Service") ||
		!strings.Contains(service, "`inject:\"true\"`") {
		t.Fatalf("service.go content did not include expected package, marker, and inject tag:\n%s", service)
	}
	repository := readFile(t, filepath.Join(root, "users", "repository.go"))
	if !strings.Contains(repository, "helix.Repository") {
		t.Fatalf("repository.go content did not include repository marker:\n%s", repository)
	}
	controller := readFile(t, filepath.Join(root, "users", "controller.go"))
	if !strings.Contains(controller, "helix.Controller") || !strings.Contains(controller, "web.Context") {
		t.Fatalf("controller.go content did not include controller marker and web context:\n%s", controller)
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test ./... error = %v\n%s", err, output)
	}
}

func TestGenerateModuleRefusesOverwrite(t *testing.T) {
	t.Parallel()

	root := newGoModuleFixture(t)
	usersDir := filepath.Join(root, "users")
	if err := os.MkdirAll(usersDir, 0o755); err != nil {
		t.Fatalf("mkdir users: %v", err)
	}
	existingPath := filepath.Join(usersDir, "service.go")
	if err := os.WriteFile(existingPath, []byte("package users\n"), 0o644); err != nil {
		t.Fatalf("write existing service: %v", err)
	}

	err := GenerateModule(ModuleOptions{RootDir: root, Name: "user"})
	if err == nil {
		t.Fatal("GenerateModule() error = nil, want existing file error")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("GenerateModule() error = %q, want 'refusing to overwrite'", err)
	}
	if got := readFile(t, existingPath); got != "package users\n" {
		t.Fatalf("existing file was overwritten: %q", got)
	}
}

func TestGenerateContextCreatesAccountsPackage(t *testing.T) {
	t.Parallel()

	root := newGoModuleFixture(t)

	if err := GenerateContext(ContextOptions{RootDir: root, Name: "accounts"}); err != nil {
		t.Fatalf("GenerateContext() error = %v", err)
	}

	for _, name := range []string{
		filepath.Join("accounts", "api.go"),
		filepath.Join("accounts", "repository.go"),
		filepath.Join("accounts", "service.go"),
		filepath.Join("accounts", "controller.go"),
	} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("generated file %s stat error = %v", name, err)
		}
	}

	api := readFile(t, filepath.Join(root, "accounts", "api.go"))
	for _, want := range []string{
		"package accounts",
		"func CreateAccount(ctx context.Context, attrs CreateAccountAttrs) (*Account, error)",
		"func GetAccount(ctx context.Context, id AccountID) (*Account, error)",
	} {
		if !strings.Contains(api, want) {
			t.Fatalf("api.go content missing %q:\n%s", want, api)
		}
	}

	service := readFile(t, filepath.Join(root, "accounts", "service.go"))
	for _, want := range []string{
		"type AccountService struct",
		"helix.Service",
		"Repository *AccountRepository `inject:\"true\"`",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("service.go content missing %q:\n%s", want, service)
		}
	}

	repository := readFile(t, filepath.Join(root, "accounts", "repository.go"))
	if !strings.Contains(repository, "type AccountRepository struct") || !strings.Contains(repository, "helix.Repository") {
		t.Fatalf("repository.go content did not include AccountRepository and repository marker:\n%s", repository)
	}

	controller := readFile(t, filepath.Join(root, "accounts", "controller.go"))
	if !strings.Contains(controller, "type AccountController struct") ||
		!strings.Contains(controller, "helix.Controller") ||
		!strings.Contains(controller, "Service *AccountService `inject:\"true\"`") ||
		!strings.Contains(controller, "web.Context") {
		t.Fatalf("controller.go content did not include expected controller shape:\n%s", controller)
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test ./... error = %v\n%s", err, output)
	}
}

func TestGenerateContextRefusesOverwrite(t *testing.T) {
	t.Parallel()

	root := newGoModuleFixture(t)
	accountsDir := filepath.Join(root, "accounts")
	if err := os.MkdirAll(accountsDir, 0o755); err != nil {
		t.Fatalf("mkdir accounts: %v", err)
	}
	existingPath := filepath.Join(accountsDir, "api.go")
	if err := os.WriteFile(existingPath, []byte("package accounts\n"), 0o644); err != nil {
		t.Fatalf("write existing api: %v", err)
	}

	err := GenerateContext(ContextOptions{RootDir: root, Name: "accounts"})
	if err == nil {
		t.Fatal("GenerateContext() error = nil, want existing file error")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("GenerateContext() error = %q, want 'refusing to overwrite'", err)
	}
	if got := readFile(t, existingPath); got != "package accounts\n" {
		t.Fatalf("existing file was overwritten: %q", got)
	}
}

func TestGenerateContextRejectsInvalidNames(t *testing.T) {
	t.Parallel()

	root := newGoModuleFixture(t)
	tests := []string{"", ".", "..", "/abs", "nested/context", "nested\\context", "123account", "account_context", "account-"}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			err := GenerateContext(ContextOptions{RootDir: root, Name: tt})
			if err == nil {
				t.Fatalf("GenerateContext(%q) error = nil, want invalid name error", tt)
			}
		})
	}
}

func TestGenerateModuleRejectsInvalidNames(t *testing.T) {
	t.Parallel()

	root := newGoModuleFixture(t)
	tests := []string{"", ".", "..", "/abs", "nested/module", "nested\\module", "123user", "user_profile", "user-"}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			err := GenerateModule(ModuleOptions{RootDir: root, Name: tt})
			if err == nil {
				t.Fatalf("GenerateModule(%q) error = nil, want invalid name error", tt)
			}
		})
	}
}

func newGoModuleFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	repoRoot := findRepoRoot(t)
	repoGoMod, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		t.Fatalf("read repo go.mod: %v", err)
	}
	content := "module example.test/app\n\ngo 1.21.0\n\nrequire github.com/enokdev/helix v0.0.0\n" +
		extractRequireBlocks(string(repoGoMod)) +
		"\nreplace github.com/enokdev/helix => " + filepath.ToSlash(repoRoot) + "\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	goSum, err := os.ReadFile(filepath.Join(repoRoot, "go.sum"))
	if err != nil {
		t.Fatalf("read repo go.sum: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.sum"), goSum, 0o644); err != nil {
		t.Fatalf("write go.sum: %v", err)
	}
	return root
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		content, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.Contains(string(content), "module github.com/enokdev/helix") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root not found")
		}
		dir = parent
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
