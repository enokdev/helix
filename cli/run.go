package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/enokdev/helix/cli/internal/runner"
)

// RunOptions configures the helix run entry point.
type RunOptions struct {
	Dir  string
	Args []string
}

// Run starts the application with hot reload enabled.
func Run(ctx context.Context, opts RunOptions) error {
	if ctx == nil {
		return fmt.Errorf("cli: run: nil context")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cli: run: %w", err)
	}
	if err := runner.Watch(ctx, runner.WatchOptions{
		RootDir: opts.Dir,
		Args:    opts.Args,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}); err != nil {
		return fmt.Errorf("cli: run: %w", err)
	}
	return nil
}
