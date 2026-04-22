package cli

import (
	"context"
	"fmt"

	"github.com/enokdev/helix/cli/internal/scaffold"
)

// GenerateContextOptions configures the helix generate context entry point.
type GenerateContextOptions struct {
	Dir  string
	Name string
}

// GenerateContext creates a DDD-light Helix context scaffold.
func GenerateContext(ctx context.Context, opts GenerateContextOptions) error {
	if ctx == nil {
		return fmt.Errorf("cli: generate context %s: nil context", opts.Name)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cli: generate context %s: %w", opts.Name, err)
	}
	if err := scaffold.GenerateContext(scaffold.ContextOptions{
		RootDir: opts.Dir,
		Name:    opts.Name,
	}); err != nil {
		return fmt.Errorf("cli: generate context %s: %w", opts.Name, err)
	}
	return nil
}
