package builder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/enokdev/helix/cli/internal/codegen"
)

const dockerfileTemplate = `# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o bin/app ./cmd/...

# Runtime stage
FROM scratch
WORKDIR /app
COPY --from=builder /app/bin/app /app
ENTRYPOINT ["/app"]
`

// BuildOptions configures the build flow.
type BuildOptions struct {
	RootDir string
	Docker  bool
	Stdout  io.Writer
	Stderr  io.Writer

	generate     func(context.Context, string) error
	buildBinary  func(context.Context, string, string, io.Writer, io.Writer) error
	writeNewFile func(string, []byte) error
	mkdirAll     func(string, os.FileMode) error
}

// Build generates code, compiles ./cmd/... into bin/app, and optionally writes a Dockerfile.
func Build(ctx context.Context, opts BuildOptions) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	root, err := projectRoot(opts.RootDir)
	if err != nil {
		return fmt.Errorf("builder: %w", err)
	}
	generate := opts.generate
	if generate == nil {
		generate = defaultGenerate
	}
	buildBinary := opts.buildBinary
	if buildBinary == nil {
		buildBinary = defaultBuildBinary
	}
	writeNewFile := opts.writeNewFile
	if writeNewFile == nil {
		writeNewFile = writeFile
	}
	mkdirAll := opts.mkdirAll
	if mkdirAll == nil {
		mkdirAll = os.MkdirAll
	}
	if err := generate(ctx, root); err != nil {
		return fmt.Errorf("builder: generate: %w", err)
	}
	if _, err := os.Stat(filepath.Join(root, "cmd")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("builder: build: cmd directory not found")
		}
		return fmt.Errorf("builder: build: stat cmd: %w", err)
	}
	// Validate that cmd directory has Go files
	if err := validateCmdHasGoFiles(root); err != nil {
		return fmt.Errorf("builder: build: %w", err)
	}
	outputPath, err := safeJoin(root, filepath.Join("bin", "app"))
	if err != nil {
		return fmt.Errorf("builder: build: %w", err)
	}
	if err := mkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("builder: build: create bin directory: %w", err)
	}
	if err := buildBinary(ctx, root, outputPath, opts.Stdout, opts.Stderr); err != nil {
		return fmt.Errorf("builder: build: %w", err)
	}
	if opts.Docker {
		dockerfilePath, err := safeJoin(root, "Dockerfile")
		if err != nil {
			return fmt.Errorf("builder: dockerfile: %w", err)
		}
		if err := writeNewFile(dockerfilePath, []byte(dockerfileTemplate)); err != nil {
			return fmt.Errorf("builder: dockerfile: %w", err)
		}
	}
	return nil
}

func defaultGenerate(ctx context.Context, root string) error {
	_, err := codegen.NewGenerator(root).Generate(ctx)
	return err
}

func defaultBuildBinary(ctx context.Context, root, outputPath string, stdout, stderr io.Writer) error {
	cmd := newBuildCmd(ctx, root, outputPath)
	cmd.Stdout = output(stdout)
	cmd.Stderr = output(stderr)
	return cmd.Run()
}

func newBuildCmd(ctx context.Context, root, outputPath string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "go", "build", "-o", outputPath, "./cmd/...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	return cmd
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

func safeJoin(root, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", fmt.Errorf("invalid path %q", name)
	}
	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	joined := filepath.Join(cleanRoot, filepath.Clean(name))
	rel, err := filepath.Rel(cleanRoot, joined)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("path %q escapes root %q", name, root)
	}
	return joined, nil
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("refusing to overwrite existing file %s", filepath.Base(path))
		}
		return err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func validateCmdHasGoFiles(root string) error {
	cmdDir := filepath.Join(root, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return fmt.Errorf("read cmd directory: %w", err)
	}
	hasGoFiles := false
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			hasGoFiles = true
			break
		}
		// Also check subdirectories for main.go
		if entry.IsDir() {
			subEntries, err := os.ReadDir(filepath.Join(cmdDir, entry.Name()))
			if err != nil {
				continue
			}
			for _, subEntry := range subEntries {
				if subEntry.Name() == "main.go" {
					hasGoFiles = true
					break
				}
			}
			if hasGoFiles {
				break
			}
		}
	}
	if !hasGoFiles {
		return fmt.Errorf("cmd directory contains no Go files (expected .go files or cmd/*/main.go)")
	}
	return nil
}

func output(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
