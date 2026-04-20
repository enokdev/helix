package cli

import (
	"context"

	"github.com/enokdev/helix/cli/internal/codegen"
)

// GenerateOptions configures the minimal helix generate entry point.
type GenerateOptions struct {
	Dir string
}

// Generate runs Helix code generation for the configured directory tree.
func Generate(ctx context.Context, opts GenerateOptions) error {
	_, err := codegen.NewGenerator(opts.Dir).Generate(ctx)
	return err
}
