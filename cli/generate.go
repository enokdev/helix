package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/enokdev/helix/cli/internal/codegen"
)

// GenerateOptions configures the minimal helix generate entry point.
type GenerateOptions struct {
	Dir    string
	Output io.Writer
}

// GenerateWireOptions configures the helix generate wire entry point.
type GenerateWireOptions struct {
	Dir string
}

// GenerateOpenAPIOptions configures the helix generate openapi entry point.
type GenerateOpenAPIOptions struct {
	Dir    string
	Output string
}

// Generate runs Helix code generation for the configured directory tree.
func Generate(ctx context.Context, opts GenerateOptions) error {
	result, err := codegen.NewGenerator(opts.Dir).Generate(ctx)
	if err != nil {
		return err
	}
	out := opts.Output
	if out == nil {
		return nil
	}
	if len(result.Files) == 0 {
		_, err = fmt.Fprintln(out, "no generated file changes")
		return err
	}
	root := opts.Dir
	if root == "" {
		root = "."
	}
	for _, file := range result.Files {
		display := filepath.ToSlash(file)
		if rel, relErr := filepath.Rel(root, file); relErr == nil && !strings.HasPrefix(rel, "..") {
			display = filepath.ToSlash(rel)
		}
		if _, err := fmt.Fprintf(out, "generated %s\n", display); err != nil {
			return err
		}
	}
	return nil
}

// GenerateWire runs compile-time DI wiring generation for the configured tree.
func GenerateWire(ctx context.Context, opts GenerateWireOptions) error {
	return codegen.NewDIGenerator(opts.Dir).Generate(ctx)
}

// GenerateOpenAPI writes an OpenAPI document for the configured tree.
func GenerateOpenAPI(ctx context.Context, opts GenerateOpenAPIOptions) error {
	_, err := codegen.NewOpenAPIGenerator(opts.Dir, opts.Output).Generate(ctx)
	return err
}
