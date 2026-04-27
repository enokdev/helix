package cli

import (
	"context"
	"fmt"

	"github.com/enokdev/helix/cli/internal/scaffold"
)

// GenerateModuleOptions configures the helix generate module entry point.
type GenerateModuleOptions struct {
	Dir  string
	Name string
}

// GenerateModule creates a conventional Helix module scaffold.
func GenerateModule(ctx context.Context, opts GenerateModuleOptions) error {
	if ctx == nil {
		return fmt.Errorf("cli: generate module %s: nil context", opts.Name)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cli: generate module %s: %w", opts.Name, err)
	}
	if err := scaffold.GenerateModule(scaffold.ModuleOptions{
		RootDir: opts.Dir,
		Name:    opts.Name,
	}); err != nil {
		return fmt.Errorf("cli: generate module %s: %w", opts.Name, err)
	}
	return nil
}
