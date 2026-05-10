package cli

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/enokdev/helix/cli/internal/scaffold"
)

// NewAppOptions configures the helix new app entry point.
type NewAppOptions struct {
	Dir              string
	Name             string
	HelixReplacePath string
	HelixVersion     string
}

// NewApp creates a minimal Helix application scaffold.
func NewApp(ctx context.Context, opts NewAppOptions) error {
	if ctx == nil {
		return fmt.Errorf("cli: new app %s: nil context", opts.Name)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cli: new app %s: %w", opts.Name, err)
	}
	version := opts.HelixVersion
	if version == "" && opts.HelixReplacePath == "" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	if err := scaffold.NewApp(scaffold.Options{
		RootDir:          opts.Dir,
		Name:             opts.Name,
		HelixReplacePath: opts.HelixReplacePath,
		HelixVersion:     version,
	}); err != nil {
		return fmt.Errorf("cli: new app %s: %w", opts.Name, err)
	}
	return nil
}
