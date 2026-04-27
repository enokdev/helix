package cli

import (
	"context"
	"fmt"

	"github.com/enokdev/helix/cli/internal/scaffold"
)

// NewAppOptions configures the helix new app entry point.
type NewAppOptions struct {
	Dir              string
	Name             string
	HelixReplacePath string
}

// NewApp creates a minimal Helix application scaffold.
func NewApp(ctx context.Context, opts NewAppOptions) error {
	if ctx == nil {
		return fmt.Errorf("cli: new app %s: nil context", opts.Name)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cli: new app %s: %w", opts.Name, err)
	}
	if err := scaffold.NewApp(scaffold.Options{
		RootDir:          opts.Dir,
		Name:             opts.Name,
		HelixReplacePath: opts.HelixReplacePath,
	}); err != nil {
		return fmt.Errorf("cli: new app %s: %w", opts.Name, err)
	}
	return nil
}
