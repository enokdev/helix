package builder

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRequiresGoMod(t *testing.T) {
	t.Parallel()

	err := Build(context.Background(), BuildOptions{RootDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "go.mod not found") {
		t.Fatalf("Build() error = %v, want missing go.mod", err)
	}
}

func TestBuildCreatesBinaryAndDockerfile(t *testing.T) {
	root := newRunnableModule(t)

	if err := Build(context.Background(), BuildOptions{RootDir: root, Docker: true}); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "bin", "app")); err != nil {
		t.Fatalf("bin/app stat error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	dockerfile := string(content)
	for _, want := range []string{
		"FROM golang:1.21-alpine AS builder",
		"RUN CGO_ENABLED=0 go build -o bin/app ./cmd/...",
		"FROM scratch",
		`ENTRYPOINT ["/app"]`,
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("Dockerfile missing %q:\n%s", want, dockerfile)
		}
	}
}

func TestBuildRefusesToOverwriteDockerfile(t *testing.T) {
	t.Parallel()

	root := newRunnableModule(t)
	if err := os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM busybox\n"), 0o644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}

	err := Build(context.Background(), BuildOptions{
		RootDir: root,
		Docker:  true,
		generate: func(context.Context, string) error {
			return nil
		},
		buildBinary: func(context.Context, string, string, io.Writer, io.Writer) error {
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite existing file Dockerfile") {
		t.Fatalf("Build() error = %v, want Dockerfile overwrite refusal", err)
	}
}

func TestNewBuildCmdSetsStaticBuildEnvironment(t *testing.T) {
	t.Parallel()

	cmd := newBuildCmd(context.Background(), "/tmp/project", "/tmp/project/bin/app")
	if cmd.Dir != "/tmp/project" {
		t.Fatalf("cmd.Dir = %q, want /tmp/project", cmd.Dir)
	}
	if got := strings.Join(cmd.Args, " "); got != "go build -o /tmp/project/bin/app ./cmd/..." {
		t.Fatalf("cmd.Args = %q", got)
	}
	if !containsEnv(cmd.Env, "CGO_ENABLED=0") {
		t.Fatalf("cmd.Env = %v, want CGO_ENABLED=0", cmd.Env)
	}
}

func newRunnableModule(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "go.mod"), "module example.test/buildfixture\n\ngo 1.21.0\n")
	writeFixtureFile(t, filepath.Join(root, "cmd", "app", "main.go"), `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`)
	return root
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func containsEnv(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}
