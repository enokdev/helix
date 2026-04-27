package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/enokdev/helix/cli/internal/builder"
)

// BuildOptions configures the helix build entry point.
type BuildOptions struct {
	Dir    string
	Docker bool
}

// Build generates code and produces a static application binary.
func Build(ctx context.Context, opts BuildOptions) error {
	if ctx == nil {
		return fmt.Errorf("cli: build: nil context")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cli: build: %w", err)
	}
	if err := builder.Build(ctx, builder.BuildOptions{
		RootDir: opts.Dir,
		Docker:  opts.Docker,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}); err != nil {
		return fmt.Errorf("cli: build: %w", err)
	}
	return nil
}
