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

// NewAPIApp creates a CRUD API Helix application scaffold.
func NewAPIApp(ctx context.Context, opts NewAppOptions) error {
	if ctx == nil {
		return fmt.Errorf("cli: new api %s: nil context", opts.Name)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cli: new api %s: %w", opts.Name, err)
	}
	version := resolveHelixVersion(opts)
	if err := scaffold.NewAPIApp(scaffold.Options{
		RootDir:          opts.Dir,
		Name:             opts.Name,
		HelixReplacePath: opts.HelixReplacePath,
		HelixVersion:     version,
	}); err != nil {
		return fmt.Errorf("cli: new api %s: %w", opts.Name, err)
	}
	return nil
}

// NewSecuredAPIApp creates a JWT-secured API Helix application scaffold.
func NewSecuredAPIApp(ctx context.Context, opts NewAppOptions) error {
	if ctx == nil {
		return fmt.Errorf("cli: new secured-api %s: nil context", opts.Name)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cli: new secured-api %s: %w", opts.Name, err)
	}
	version := resolveHelixVersion(opts)
	if err := scaffold.NewSecuredAPIApp(scaffold.Options{
		RootDir:          opts.Dir,
		Name:             opts.Name,
		HelixReplacePath: opts.HelixReplacePath,
		HelixVersion:     version,
	}); err != nil {
		return fmt.Errorf("cli: new secured-api %s: %w", opts.Name, err)
	}
	return nil
}

// NewGORMAPIApp creates a GORM/SQLite API Helix application scaffold.
func NewGORMAPIApp(ctx context.Context, opts NewAppOptions) error {
	if ctx == nil {
		return fmt.Errorf("cli: new gorm-api %s: nil context", opts.Name)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cli: new gorm-api %s: %w", opts.Name, err)
	}
	version := resolveHelixVersion(opts)
	if err := scaffold.NewGORMAPIApp(scaffold.Options{
		RootDir:          opts.Dir,
		Name:             opts.Name,
		HelixReplacePath: opts.HelixReplacePath,
		HelixVersion:     version,
	}); err != nil {
		return fmt.Errorf("cli: new gorm-api %s: %w", opts.Name, err)
	}
	return nil
}

func resolveHelixVersion(opts NewAppOptions) string {
	if opts.HelixVersion != "" || opts.HelixReplacePath != "" {
		return opts.HelixVersion
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return ""
}
